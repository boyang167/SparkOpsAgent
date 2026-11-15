package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

type CompleteTransferMultipart struct {
	SessionID   uuid.UUID
	WorkspaceID uuid.UUID
}

type CompleteTransferMultipartHandler struct {
	repo      repository.FilesystemRepository
	providers StorageProviderCatalog
	recorded  *RecordTransferUploadedHandler
}

func NewCompleteTransferMultipartHandler(
	repo repository.FilesystemRepository,
	providers StorageProviderCatalog,
	recorded *RecordTransferUploadedHandler,
) *CompleteTransferMultipartHandler {
	if repo == nil || providers == nil || recorded == nil {
		panic("filesystem complete transfer multipart requires repository, providers and record handler")
	}
	return &CompleteTransferMultipartHandler{repo: repo, providers: providers, recorded: recorded}
}

func (h *CompleteTransferMultipartHandler) Handle(ctx context.Context, cmd CompleteTransferMultipart) error {
	if cmd.SessionID == uuid.Nil {
		return model.ErrInvalidTransfer
	}
	session, err := h.repo.GetTransferByID(ctx, cmd.SessionID)
	if err != nil {
		return err
	}
	if cmd.WorkspaceID != uuid.Nil && session.WorkspaceID() != cmd.WorkspaceID {
		return model.ErrInvalidTransfer
	}
	if session.Protocol() != model.TransferProtocolMultipart {
		return model.ErrInvalidTransfer
	}
	switch session.Status() {
	case model.TransferCompleted, model.TransferRejected, model.TransferFailed,
		model.TransferAborted, model.TransferExpired:
		return nil
	case model.TransferScanning, model.TransferVerifying, model.TransferCommitting, model.TransferUploading:
		return nil
	case model.TransferInitiated, model.TransferPaused:
	default:
		return model.ErrInvalidTransferTransition
	}
	uploadID, err := h.repo.TransferProviderUploadID(ctx, cmd.SessionID)
	if err != nil {
		return err
	}
	uploadID = strings.TrimSpace(uploadID)
	if uploadID == "" {
		return model.ErrInvalidTransfer
	}
	providerID := session.ProviderID()
	if providerID == nil {
		return model.ErrInvalidTransfer
	}
	provider, err := h.providers.Provider(ctx, *providerID)
	if err != nil {
		return err
	}
	multipart, ok := provider.(storage.MultipartProvider)
	if !ok {
		return model.ErrInvalidTransfer
	}
	uploaded, err := multipart.ListParts(ctx, uploadID)
	if err != nil {
		return err
	}
	partSize, totalParts := transferPartLayout(session)
	if totalParts > 0 && len(uploaded) < totalParts {
		return model.ErrInvalidTransferPart
	}
	completed := make([]storage.CompletedPart, 0, len(uploaded))
	for _, part := range uploaded {
		if part.PartNumber <= 0 || strings.TrimSpace(part.ETag) == "" {
			return model.ErrInvalidTransferPart
		}
		completed = append(completed, storage.CompletedPart{PartNumber: part.PartNumber, ETag: part.ETag})
		domainPart, partErr := model.NewTransferPart(model.TransferPartInput{
			ID: uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("filesystem-transfer-part:%s:%d", cmd.SessionID, part.PartNumber))),
			SessionID: cmd.SessionID, PartNumber: part.PartNumber,
			Offset: func() int64 {
				off, _ := transferPartBounds(session.ExpectedSize(), partSize, part.PartNumber)
				return off
			}(),
			Size: part.Size, SessionSize: session.ExpectedSize(), ETag: part.ETag,
		})
		if partErr != nil {
			return partErr
		}
		if err := h.repo.UpsertTransferPart(ctx, domainPart); err != nil {
			return err
		}
	}
	info, err := multipart.CompleteMultipart(ctx, uploadID, completed)
	if err != nil {
		return err
	}
	checksum := strings.ToLower(strings.TrimSpace(info.SHA256))
	return h.recorded.Handle(ctx, RecordTransferUploaded{
		SessionID: cmd.SessionID, WorkspaceID: session.WorkspaceID(),
		Size: info.Size, SHA256: checksum,
	})
}
package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

const transferPartETagPending = "pending"

type PrepareTransferMultipartHandler struct {
	repo      repository.FilesystemRepository
	providers StorageProviderCatalog
}

func NewPrepareTransferMultipartHandler(repo repository.FilesystemRepository, providers StorageProviderCatalog) *PrepareTransferMultipartHandler {
	if repo == nil || providers == nil {
		panic("filesystem prepare transfer multipart requires repository and providers")
	}
	return &PrepareTransferMultipartHandler{repo: repo, providers: providers}
}

func (h *PrepareTransferMultipartHandler) Handle(ctx context.Context, sessionID uuid.UUID) error {
	if sessionID == uuid.Nil {
		return model.ErrInvalidTransfer
	}
	session, err := h.repo.GetTransferByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.Protocol() != model.TransferProtocolMultipart {
		return nil
	}
	existing, err := h.repo.TransferProviderUploadID(ctx, sessionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(existing) != "" {
		return nil
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
		return domain.ErrStorageGatewayMismatch
	}
	upload, err := multipart.CreateMultipart(ctx, session.TemporaryObjectKey(), session.ExpectedSize())
	if err != nil {
		return err
	}
	if err := h.repo.SetTransferProviderUploadID(ctx, sessionID, upload.UploadID); err != nil {
		return err
	}
	partSize, totalParts := transferPartLayout(session)
	for partNumber := 1; partNumber <= totalParts; partNumber++ {
		offset, size := transferPartBounds(session.ExpectedSize(), partSize, partNumber)
		if size <= 0 {
			continue
		}
		part, partErr := model.NewTransferPart(model.TransferPartInput{
			ID: uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("filesystem-transfer-part:%s:%d", sessionID, partNumber))),
			SessionID: sessionID, PartNumber: partNumber, Offset: offset, Size: size,
			SessionSize: session.ExpectedSize(), ETag: transferPartETagPending,
		})
		if partErr != nil {
			return partErr
		}
		if err := h.repo.UpsertTransferPart(ctx, part); err != nil {
			return err
		}
	}
	return nil
}

func transferPartLayout(session *model.TransferSession) (partSize int64, totalParts int) {
	partSize = defaultPartSize
	if session.ExpectedSize() < partSize {
		partSize = session.ExpectedSize()
	}
	if partSize <= 0 {
		return partSize, 0
	}
	totalParts = int((session.ExpectedSize() + partSize - 1) / partSize)
	return partSize, totalParts
}

func transferPartBounds(expectedSize, partSize int64, partNumber int) (offset, size int64) {
	offset = int64(partNumber-1) * partSize
	size = partSize
	if offset+size > expectedSize {
		size = expectedSize - offset
	}
	return offset, size
}
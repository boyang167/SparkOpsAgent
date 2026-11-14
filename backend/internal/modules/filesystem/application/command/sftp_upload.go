package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type BeginSFTPUpload struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	ActorID        uuid.UUID
	RelPath        string
	SessionPath    string
	Size           int64
	SHA256         string
}

type BeginSFTPUploadResult struct {
	SessionID          uuid.UUID
	TemporaryObjectKey string
	StorageRoot        string
}

type BeginSFTPUploadHandler struct {
	repo         repository.FilesystemRepository
	create       *CreateTransferHandler
	storageRoot  string
	ensureLocal  *EnsureDefaultLocalProviderHandler
}

func NewBeginSFTPUploadHandler(
	repo repository.FilesystemRepository,
	create *CreateTransferHandler,
	storageRoot string,
	ensure *EnsureDefaultLocalProviderHandler,
) *BeginSFTPUploadHandler {
	if create == nil || repo == nil {
		panic("sftp begin upload requires repository and create transfer")
	}
	return &BeginSFTPUploadHandler{
		repo: repo, create: create, storageRoot: strings.TrimSpace(storageRoot), ensureLocal: ensure,
	}
}

func (h *BeginSFTPUploadHandler) Handle(ctx context.Context, cmd BeginSFTPUpload) (BeginSFTPUploadResult, error) {
	name := strings.TrimSpace(cmd.RelPath)
	name = strings.ReplaceAll(name, "\\", "/")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if name == "" || cmd.OrganizationID == uuid.Nil || cmd.WorkspaceID == uuid.Nil || cmd.ActorID == uuid.Nil {
		return BeginSFTPUploadResult{}, model.ErrInvalidTransfer
	}
	ns, err := h.repo.GetNamespaceByWorkspace(ctx, cmd.WorkspaceID)
	if err != nil {
		return BeginSFTPUploadResult{}, err
	}
	if ns.OrganizationID != cmd.OrganizationID {
		return BeginSFTPUploadResult{}, domain.ErrNamespaceMismatch
	}
	providerID := uuid.Nil
	if h.ensureLocal != nil && h.storageRoot != "" {
		providerID, err = h.ensureLocal.Handle(ctx, EnsureDefaultLocalProvider{
			OrganizationID: cmd.OrganizationID,
			StorageRoot:    h.storageRoot,
		})
		if err != nil {
			return BeginSFTPUploadResult{}, err
		}
	} else {
		providerID, err = h.create.defaultProviderID(ctx, cmd.OrganizationID)
		if err != nil {
			return BeginSFTPUploadResult{}, err
		}
	}
	idem := uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf(
		"sftp-upload:%s:%s:%s", cmd.WorkspaceID, cmd.SessionPath, cmd.RelPath,
	))).String()
	descriptor, err := h.create.Handle(ctx, CreateTransfer{
		ActorID: cmd.ActorID, OrganizationID: cmd.OrganizationID, WorkspaceID: cmd.WorkspaceID,
		NamespaceID: ns.ID, ParentID: ns.RootNodeID, Name: name,
		Protocol: model.TransferProtocolDirect, Size: cmd.Size,
		SHA256: strings.ToLower(strings.TrimSpace(cmd.SHA256)),
		Classification: model.TransferClassificationInternal,
		IdempotencyKey: idem, ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		ProviderID: providerID,
		Authorization:  Authorization{Allowed: true},
	})
	if err != nil {
		return BeginSFTPUploadResult{}, err
	}
	session, err := h.repo.GetTransferByID(ctx, descriptor.SessionID)
	if err != nil {
		return BeginSFTPUploadResult{}, err
	}
	return BeginSFTPUploadResult{
		SessionID:          descriptor.SessionID,
		TemporaryObjectKey: session.TemporaryObjectKey(),
		StorageRoot:        h.storageRoot,
	}, nil
}
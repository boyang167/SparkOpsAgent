package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
)

type GetFileStorage struct {
	NamespaceID uuid.UUID
	NodeID      uuid.UUID
	RevisionID  *uuid.UUID
}

type FileStorageView struct {
	RevisionID      uuid.UUID
	BlobID          uuid.UUID
	ProviderID      uuid.UUID
	ProviderType    model.ProviderType
	ProviderRoot    string
	ObjectKey       string
	ContentSHA256   string
	LocalObjectFile string
}

type fileStorageReader interface {
	GetNode(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	GetRevision(context.Context, uuid.UUID, uuid.UUID) (*model.FileRevision, error)
	GetBlob(context.Context, uuid.UUID) (*model.Blob, error)
	GetPreferredReplica(context.Context, uuid.UUID) (*model.StorageReplica, error)
	GetStorageProvider(context.Context, uuid.UUID) (*model.StorageProvider, error)
}

type GetFileStorageHandler struct {
	repo fileStorageReader
}

func NewGetFileStorageHandler(repo repository.FilesystemRepository) *GetFileStorageHandler {
	return &GetFileStorageHandler{repo: repo}
}

func (h *GetFileStorageHandler) Handle(ctx context.Context, q GetFileStorage) (FileStorageView, error) {
	var empty FileStorageView
	if h == nil || h.repo == nil {
		return empty, domain.ErrPersistence
	}
	node, err := h.repo.GetNode(ctx, q.NamespaceID, q.NodeID)
	if err != nil {
		return empty, err
	}
	if node.Type != model.NodeTypeFile {
		return empty, domain.ErrUnsupportedNodeType
	}
	revisionID := node.CurrentRevisionID
	if q.RevisionID != nil {
		revisionID = q.RevisionID
	}
	if revisionID == nil {
		return empty, domain.ErrRevisionNotFound
	}
	revision, err := h.repo.GetRevision(ctx, q.NodeID, *revisionID)
	if err != nil {
		return empty, err
	}
	blob, err := h.repo.GetBlob(ctx, revision.BlobID())
	if err != nil {
		return empty, err
	}
	replica, err := h.repo.GetPreferredReplica(ctx, blob.ID)
	if err != nil {
		return empty, err
	}
	provider, err := h.repo.GetStorageProvider(ctx, replica.ProviderID)
	if err != nil {
		return empty, err
	}
	view := FileStorageView{
		RevisionID:    *revisionID,
		BlobID:        blob.ID,
		ProviderID:    provider.ID,
		ProviderType:  provider.ProviderType,
		ProviderRoot:  provider.Endpoint,
		ObjectKey:     replica.ObjectKey,
		ContentSHA256: blob.SHA256,
	}
	if provider.ProviderType == model.ProviderTypeLocal {
		view.LocalObjectFile = local.ObjectFilePath(provider.Endpoint, replica.ObjectKey)
	}
	return view, nil
}
package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type PreviewBlobReader struct {
	repo repository.FilesystemRepository
}

func NewPreviewBlobReader(repo repository.FilesystemRepository) *PreviewBlobReader {
	return &PreviewBlobReader{repo: repo}
}

func (r *PreviewBlobReader) BlobID(ctx context.Context, _, nodeID uuid.UUID, revisionID *uuid.UUID) (uuid.UUID, error) {
	if r == nil || r.repo == nil {
		return uuid.Nil, nil
	}
	link, err := r.repo.GetFileLink(ctx, nodeID, revisionID, model.FileLinkTypePreview)
	if err != nil {
		return uuid.Nil, err
	}
	return link.LinkID, nil
}

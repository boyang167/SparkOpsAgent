package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type ListArtifacts struct {
	WorkspaceID uuid.UUID
	RunID       *uuid.UUID
	Limit       int
	Cursor      string
}

type ListArtifactsHandler struct {
	repo repository.FilesystemRepository
}

func NewListArtifactsHandler(repo repository.FilesystemRepository) *ListArtifactsHandler {
	return &ListArtifactsHandler{repo: repo}
}

func (h *ListArtifactsHandler) Handle(ctx context.Context, q ListArtifacts) ([]model.Artifact, string, error) {
	if h == nil || h.repo == nil {
		return nil, "", nil
	}
	return h.repo.ListArtifacts(ctx, q.WorkspaceID, q.RunID, q.Limit, q.Cursor)
}

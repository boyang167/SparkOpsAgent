package command

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type PromoteReplica struct {
	BlobID           uuid.UUID
	SourceProviderID uuid.UUID
	TargetProviderID uuid.UUID
	DrainingGrace    time.Duration
}

type PromoteReplicaHandler struct {
	repo repository.FilesystemRepository
}

func NewPromoteReplicaHandler(repo repository.FilesystemRepository, _ any) *PromoteReplicaHandler {
	return &PromoteReplicaHandler{repo: repo}
}

func (h *PromoteReplicaHandler) Handle(ctx context.Context, cmd PromoteReplica) error {
	if h.repo == nil {
		return fmt.Errorf("promote replica repository is required")
	}
	grace := cmd.DrainingGrace
	if grace <= 0 {
		grace = 7 * 24 * time.Hour
	}
	deleteAfter := time.Now().UTC().Add(grace)
	return h.repo.PromoteReplica(ctx, repository.ReplicaPromotion{
		BlobID: cmd.BlobID, SourceProviderID: cmd.SourceProviderID,
		TargetProviderID: cmd.TargetProviderID, DeleteAfter: deleteAfter,
	})
}

package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
)

type storagePoolDeleter interface {
	DeleteStoragePool(context.Context, uuid.UUID, uuid.UUID) error
}

type DeleteStoragePool struct {
	OrganizationID uuid.UUID
	PoolID         uuid.UUID
}

type DeleteStoragePoolHandler struct {
	repo storagePoolDeleter
}

func NewDeleteStoragePoolHandler(repo storagePoolDeleter) *DeleteStoragePoolHandler {
	if repo == nil {
		panic("delete storage pool requires repository")
	}
	return &DeleteStoragePoolHandler{repo: repo}
}

func (h *DeleteStoragePoolHandler) Handle(ctx context.Context, cmd DeleteStoragePool) error {
	if cmd.OrganizationID == uuid.Nil || cmd.PoolID == uuid.Nil {
		return domain.ErrStoragePoolNotFound
	}
	return h.repo.DeleteStoragePool(ctx, cmd.OrganizationID, cmd.PoolID)
}
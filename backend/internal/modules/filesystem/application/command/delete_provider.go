package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
)

type storageProviderDeleter interface {
	DeleteStorageProvider(context.Context, uuid.UUID, uuid.UUID) error
}

type DeleteProvider struct {
	OrganizationID uuid.UUID
	ProviderID     uuid.UUID
}

type DeleteProviderHandler struct {
	repo storageProviderDeleter
}

func NewDeleteProviderHandler(repo storageProviderDeleter) *DeleteProviderHandler {
	if repo == nil {
		panic("delete provider requires repository")
	}
	return &DeleteProviderHandler{repo: repo}
}

func (h *DeleteProviderHandler) Handle(ctx context.Context, cmd DeleteProvider) error {
	if cmd.OrganizationID == uuid.Nil || cmd.ProviderID == uuid.Nil {
		return domain.ErrStorageProviderNotFound
	}
	if cmd.ProviderID == DefaultLocalProviderID(cmd.OrganizationID) {
		return domain.ErrStorageProviderProtected
	}
	return h.repo.DeleteStorageProvider(ctx, cmd.OrganizationID, cmd.ProviderID)
}
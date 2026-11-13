package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type storagePoolWriter interface {
	SaveStoragePool(context.Context, *model.StoragePool, uuid.UUID) error
}

type CreateStoragePoolMember struct {
	ProviderID uuid.UUID
	Mode       model.ProviderMode
	Priority   int
}

type CreateStoragePool struct {
	CommandID      uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Members        []CreateStoragePoolMember
}

type CreateStoragePoolResult struct {
	ID   uuid.UUID
	Name string
}

type CreateStoragePoolHandler struct {
	repo storagePoolWriter
}

func NewCreateStoragePoolHandler(repo storagePoolWriter) *CreateStoragePoolHandler {
	if repo == nil {
		panic("create storage pool requires repository")
	}
	return &CreateStoragePoolHandler{repo: repo}
}

func (h *CreateStoragePoolHandler) Handle(ctx context.Context, cmd CreateStoragePool) (CreateStoragePoolResult, error) {
	pool := model.NewStoragePool(uuid.New(), cmd.Name)
	for _, member := range cmd.Members {
		if err := pool.AddProvider(member.ProviderID, member.Mode, member.Priority); err != nil {
			return CreateStoragePoolResult{}, err
		}
	}
	if err := h.repo.SaveStoragePool(ctx, pool, cmd.OrganizationID); err != nil {
		return CreateStoragePoolResult{}, err
	}
	return CreateStoragePoolResult{ID: pool.ID(), Name: pool.Name()}, nil
}

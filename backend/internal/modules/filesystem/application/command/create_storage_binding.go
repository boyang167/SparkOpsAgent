package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type storageBindingWriter interface {
	SaveStorageBinding(context.Context, *model.StorageBinding) error
}

type CreateStorageBinding struct {
	CommandID      uuid.UUID
	OrganizationID uuid.UUID
	ScopeType      model.ScopeType
	ScopeID        uuid.UUID
	PoolID         uuid.UUID
	Priority       int
}

type CreateStorageBindingResult struct {
	ID uuid.UUID
}

type CreateStorageBindingHandler struct {
	repo storageBindingWriter
}

func NewCreateStorageBindingHandler(repo storageBindingWriter) *CreateStorageBindingHandler {
	if repo == nil {
		panic("create storage binding requires repository")
	}
	return &CreateStorageBindingHandler{repo: repo}
}

func (h *CreateStorageBindingHandler) Handle(ctx context.Context, cmd CreateStorageBinding) (CreateStorageBindingResult, error) {
	binding, err := model.NewStorageBinding(model.StorageBindingInput{
		ID: uuid.New(), OrganizationID: cmd.OrganizationID,
		ScopeType: cmd.ScopeType, ScopeID: cmd.ScopeID, PoolID: cmd.PoolID, Priority: cmd.Priority,
	})
	if err != nil {
		return CreateStorageBindingResult{}, err
	}
	if err := h.repo.SaveStorageBinding(ctx, binding); err != nil {
		return CreateStorageBindingResult{}, err
	}
	return CreateStorageBindingResult{ID: binding.ID}, nil
}

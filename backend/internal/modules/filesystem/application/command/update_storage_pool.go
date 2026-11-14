package command

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type storagePoolUpdater interface {
	GetStoragePoolForOrganization(context.Context, uuid.UUID, uuid.UUID) (*model.StoragePool, error)
	SaveStoragePool(context.Context, *model.StoragePool, uuid.UUID) error
}

type UpdateStoragePool struct {
	OrganizationID uuid.UUID
	PoolID         uuid.UUID
	Name           string
	Status         model.StoragePoolStatus
	Members        []CreateStoragePoolMember
}

type UpdateStoragePoolResult struct {
	ID   uuid.UUID
	Name string
}

type UpdateStoragePoolHandler struct {
	repo storagePoolUpdater
}

func NewUpdateStoragePoolHandler(repo storagePoolUpdater) *UpdateStoragePoolHandler {
	if repo == nil {
		panic("update storage pool requires repository")
	}
	return &UpdateStoragePoolHandler{repo: repo}
}

func (h *UpdateStoragePoolHandler) Handle(ctx context.Context, cmd UpdateStoragePool) (UpdateStoragePoolResult, error) {
	pool, err := h.repo.GetStoragePoolForOrganization(ctx, cmd.OrganizationID, cmd.PoolID)
	if err != nil {
		return UpdateStoragePoolResult{}, err
	}
	snapshot := pool.Snapshot()
	snapshot.Name = strings.TrimSpace(cmd.Name)
	if snapshot.Name == "" {
		return UpdateStoragePoolResult{}, model.ErrInvalidStoragePool
	}
	if cmd.Status != "" {
		snapshot.Status = cmd.Status
	}
	snapshot.PlacementPolicyVersion++
	snapshot.Members = nil
	rebuilt := model.RestoreStoragePool(snapshot)
	for _, member := range cmd.Members {
		if addErr := rebuilt.AddProvider(member.ProviderID, member.Mode, member.Priority); addErr != nil {
			return UpdateStoragePoolResult{}, addErr
		}
	}
	if err := h.repo.SaveStoragePool(ctx, rebuilt, cmd.OrganizationID); err != nil {
		return UpdateStoragePoolResult{}, err
	}
	return UpdateStoragePoolResult{ID: rebuilt.ID(), Name: rebuilt.Name()}, nil
}
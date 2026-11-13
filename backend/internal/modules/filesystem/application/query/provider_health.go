package query

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

type providerHealthReader interface {
	GetStorageProvider(context.Context, uuid.UUID) (*model.StorageProvider, error)
}

type ProviderHealth struct {
	ProviderID uuid.UUID
}

type ProviderHealthHandler struct {
	repo     providerHealthReader
	registry *storage.Registry
}

func NewProviderHealthHandler(repo providerHealthReader, registry *storage.Registry) *ProviderHealthHandler {
	if repo == nil || registry == nil {
		panic("provider health requires repository and registry")
	}
	return &ProviderHealthHandler{repo: repo, registry: registry}
}

func (h *ProviderHealthHandler) Handle(ctx context.Context, q ProviderHealth) (storage.ProviderHealth, error) {
	var empty storage.ProviderHealth
	if q.ProviderID == uuid.Nil {
		return empty, model.ErrInvalidStorageProvider
	}
	if _, err := h.repo.GetStorageProvider(ctx, q.ProviderID); err != nil {
		return empty, err
	}
	provider, err := h.registry.Resolve(ctx, q.ProviderID)
	if err != nil {
		if errors.Is(err, storage.ErrProviderRecordNotFound) {
			return empty, domain.ErrStorageProviderNotFound
		}
		return empty, err
	}
	return storage.ProbeProviderHealth(ctx, provider)
}

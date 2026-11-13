package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

type StorageProviderCatalog interface {
	Provider(context.Context, uuid.UUID) (storage.Provider, error)
}

type MapProviders map[uuid.UUID]storage.Provider

func (m MapProviders) Provider(_ context.Context, id uuid.UUID) (storage.Provider, error) {
	if m == nil {
		return nil, fmt.Errorf("storage provider catalog is not configured")
	}
	provider, ok := m[id]
	if !ok || provider == nil {
		return nil, fmt.Errorf("storage provider %s is not registered", id)
	}
	return provider, nil
}

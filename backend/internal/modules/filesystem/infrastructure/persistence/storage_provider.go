package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

func storageProviderRowFromModel(provider *model.StorageProvider, now time.Time) storageProviderRow {
	capabilities := jsonObject(provider.Capabilities)
	if capabilities == nil {
		capabilities = jsonObject{}
	}
	health := jsonObject(provider.Health)
	if health == nil {
		health = jsonObject{}
	}
	return storageProviderRow{
		ID: provider.ID, OrganizationID: provider.OrganizationID, Name: provider.Name,
		ProviderType: provider.ProviderType, Endpoint: provider.Endpoint, SecretRef: provider.SecretRef,
		Capabilities: capabilities, Health: health, ConfigVersion: provider.ConfigVersion,
		Status: provider.Status, CreatedAt: now, UpdatedAt: now,
	}
}

func storageProviderFromRow(row storageProviderRow) *model.StorageProvider {
	capabilities := map[string]any(row.Capabilities)
	if capabilities == nil {
		capabilities = map[string]any{}
	}
	health := map[string]any(row.Health)
	if health == nil {
		health = map[string]any{}
	}
	return &model.StorageProvider{
		ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name,
		ProviderType: row.ProviderType, Endpoint: row.Endpoint, SecretRef: row.SecretRef,
		Capabilities: capabilities, Health: health, ConfigVersion: row.ConfigVersion, Status: row.Status,
	}
}

func storageProviderResult(row storageProviderRow, err error) (*model.StorageProvider, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrStorageProviderNotFound
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	return storageProviderFromRow(row), nil
}

type RegistryProviderLoader struct {
	repo *Repository
}

func NewRegistryProviderLoader(repo *Repository) *RegistryProviderLoader {
	if repo == nil {
		panic("registry provider loader requires repository")
	}
	return &RegistryProviderLoader{repo: repo}
}

func (l *RegistryProviderLoader) LoadProvider(ctx context.Context, id uuid.UUID) (*storage.ProviderRecord, error) {
	provider, err := l.repo.GetStorageProvider(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrStorageProviderNotFound) {
			return nil, storage.ErrProviderRecordNotFound
		}
		return nil, err
	}
	return &storage.ProviderRecord{
		ID: provider.ID, OrganizationID: provider.OrganizationID, Name: provider.Name,
		ProviderType: storage.ProviderType(provider.ProviderType), Endpoint: provider.Endpoint,
		SecretRef: provider.SecretRef, Capabilities: provider.Capabilities,
		ConfigVersion: provider.ConfigVersion, Status: provider.Status,
	}, nil
}

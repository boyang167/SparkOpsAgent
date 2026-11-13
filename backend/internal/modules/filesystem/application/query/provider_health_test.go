package query

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/wire"
)

type healthProviderRepo struct {
	record *model.StorageProvider
}

func (r *healthProviderRepo) GetStorageProvider(_ context.Context, id uuid.UUID) (*model.StorageProvider, error) {
	if r.record == nil || r.record.ID != id {
		return nil, domain.ErrStorageProviderNotFound
	}
	copy := *r.record
	return &copy, nil
}

type mapSecrets struct {
	values map[string]json.RawMessage
}

func (m mapSecrets) Resolve(_ context.Context, ref string) (json.RawMessage, error) {
	value, ok := m.values[ref]
	if !ok {
		return nil, storage.ErrProviderSecretUnavailable
	}
	return value, nil
}

type repoLoader struct {
	repo *healthProviderRepo
}

func (l repoLoader) LoadProvider(ctx context.Context, id uuid.UUID) (*storage.ProviderRecord, error) {
	provider, err := l.repo.GetStorageProvider(ctx, id)
	if err != nil {
		return nil, storage.ErrProviderRecordNotFound
	}
	return &storage.ProviderRecord{
		ID: provider.ID, OrganizationID: provider.OrganizationID, Name: provider.Name,
		ProviderType: storage.ProviderType(provider.ProviderType), Endpoint: provider.Endpoint,
		SecretRef: provider.SecretRef, Capabilities: provider.Capabilities,
		ConfigVersion: provider.ConfigVersion, Status: provider.Status,
	}, nil
}

func TestProviderHealthReturnsHealthyProbe(t *testing.T) {
	id := uuid.New()
	ref := "vault://health"
	root := t.TempDir()
	repo := &healthProviderRepo{record: &model.StorageProvider{
		ID: id, OrganizationID: uuid.New(), Name: "probe", ProviderType: model.ProviderTypeLocal,
		Endpoint: root, SecretRef: ref, ConfigVersion: 1, Status: "ACTIVE",
		Capabilities: map[string]any{}, Health: map[string]any{},
	}}
	registry := storage.NewRegistry(mapSecrets{values: map[string]json.RawMessage{ref: json.RawMessage(`{}`)}}, repoLoader{repo: repo}, wire.DefaultProviderBuilder)
	handler := NewProviderHealthHandler(repo, registry)
	t.Cleanup(func() { require.NoError(t, registry.Close()) })
	result, err := handler.Handle(context.Background(), ProviderHealth{ProviderID: id})
	require.NoError(t, err)
	require.Equal(t, "HEALTHY", result.Status)
	require.False(t, result.CheckedAt.IsZero())
}

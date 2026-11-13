package persistence

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

func TestStorageProviderRoundTrip(t *testing.T) {
	repo, _ := newTestRepository(t)
	id := uuid.New()
	orgID := uuid.New()
	provider, err := model.NewStorageProvider(model.StorageProviderInput{
		ID: id, OrganizationID: orgID, Name: "vault-local",
		ProviderType: model.ProviderTypeLocal, Endpoint: t.TempDir(),
		SecretRef: "vault://filesystem/local", Capabilities: map[string]any{"encryption": true},
	})
	require.NoError(t, err)
	require.NoError(t, repo.CreateStorageProvider(context.Background(), provider))
	got, err := repo.GetStorageProvider(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, provider.SecretRef, got.SecretRef)
	require.Equal(t, provider.ProviderType, got.ProviderType)
	require.Equal(t, provider.ConfigVersion, got.ConfigVersion)
	require.Equal(t, true, got.Capabilities["encryption"])
}

func TestStorageProviderRejectsDuplicateName(t *testing.T) {
	repo, _ := newTestRepository(t)
	orgID := uuid.New()
	first, err := model.NewStorageProvider(model.StorageProviderInput{
		ID: uuid.New(), OrganizationID: orgID, Name: "dup",
		ProviderType: model.ProviderTypeLocal, Endpoint: t.TempDir(), SecretRef: "vault://a",
	})
	require.NoError(t, err)
	second, err := model.NewStorageProvider(model.StorageProviderInput{
		ID: uuid.New(), OrganizationID: orgID, Name: "dup",
		ProviderType: model.ProviderTypeLocal, Endpoint: t.TempDir(), SecretRef: "vault://b",
	})
	require.NoError(t, err)
	require.NoError(t, repo.CreateStorageProvider(context.Background(), first))
	err = repo.CreateStorageProvider(context.Background(), second)
	require.ErrorIs(t, err, domain.ErrStorageProviderConflict)
}

func TestRegistryProviderLoaderMapsRecord(t *testing.T) {
	repoIface, _ := newTestRepository(t)
	concrete, ok := repoIface.(*Repository)
	require.True(t, ok)
	id := uuid.New()
	provider, err := model.NewStorageProvider(model.StorageProviderInput{
		ID: id, OrganizationID: uuid.New(), Name: "loader",
		ProviderType: model.ProviderTypeLocal, Endpoint: t.TempDir(), SecretRef: "vault://loader",
	})
	require.NoError(t, err)
	require.NoError(t, concrete.CreateStorageProvider(context.Background(), provider))
	loader := NewRegistryProviderLoader(concrete)
	record, err := loader.LoadProvider(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, provider.SecretRef, record.SecretRef)
	require.Equal(t, int64(1), record.ConfigVersion)
}

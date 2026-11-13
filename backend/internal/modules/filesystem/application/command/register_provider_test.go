package command

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type providerRepoStub struct {
	saved *model.StorageProvider
}

func (s *providerRepoStub) CreateStorageProvider(_ context.Context, provider *model.StorageProvider) error {
	saved := *provider
	s.saved = &saved
	return nil
}

func (s *providerRepoStub) GetStorageProvider(context.Context, uuid.UUID) (*model.StorageProvider, error) {
	if s.saved == nil {
		return nil, domain.ErrStorageProviderNotFound
	}
	copy := *s.saved
	return &copy, nil
}

func TestRegisterProviderPersistsMetadataWithoutPlaintextSecret(t *testing.T) {
	orgID := uuid.New()
	repo := &providerRepoStub{}
	handler := NewRegisterProviderHandler(repo)
	id := uuid.New()
	result, err := handler.Handle(context.Background(), RegisterProvider{
		ID: id, OrganizationID: orgID, Name: "local-primary",
		ProviderType: model.ProviderTypeLocal, Endpoint: t.TempDir(),
		SecretRef: "vault://filesystem/local-primary",
		Capabilities: map[string]any{"encryption": true},
	})
	require.NoError(t, err)
	require.Equal(t, id, result.ID)
	require.Equal(t, "vault://filesystem/local-primary", repo.saved.SecretRef)
	require.Equal(t, true, repo.saved.Capabilities["encryption"])
}

func TestRegisterProviderRejectsEmptySecretRef(t *testing.T) {
	handler := NewRegisterProviderHandler(&providerRepoStub{})
	_, err := handler.Handle(context.Background(), RegisterProvider{
		ID: uuid.New(), OrganizationID: uuid.New(), Name: "broken",
		ProviderType: model.ProviderTypeLocal, Endpoint: t.TempDir(), SecretRef: " ",
	})
	require.ErrorIs(t, err, model.ErrInvalidStorageProvider)
}

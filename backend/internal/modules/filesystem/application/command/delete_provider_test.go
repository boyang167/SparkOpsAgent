package command

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
)

type deleteProviderRepoStub struct {
	err error
}

func (s *deleteProviderRepoStub) DeleteStorageProvider(context.Context, uuid.UUID, uuid.UUID) error {
	return s.err
}

func TestDeleteProviderRejectsDefaultLocal(t *testing.T) {
	orgID := uuid.New()
	handler := NewDeleteProviderHandler(&deleteProviderRepoStub{})
	err := handler.Handle(context.Background(), DeleteProvider{
		OrganizationID: orgID,
		ProviderID:     DefaultLocalProviderID(orgID),
	})
	require.ErrorIs(t, err, domain.ErrStorageProviderProtected)
}
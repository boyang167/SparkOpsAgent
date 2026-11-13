package command

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	filesystemport "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
)

type grantPortStub struct {
	created int
}

func (s *grantPortStub) Create(context.Context, filesystemport.GrantRequest) (model.ResourceGrant, error) {
	s.created++
	return model.ResourceGrant{ID: uuid.New()}, nil
}

func (s *grantPortStub) Revoke(context.Context, filesystemport.GrantRequest, uuid.UUID) error {
	return nil
}

func (s *grantPortStub) List(context.Context, filesystemport.GrantRequest) ([]model.ResourceGrant, error) {
	return nil, nil
}

func TestCrossOrganizationGrantIsRejected(t *testing.T) {
	stub := &grantPortStub{}
	grantHandler := NewGrantResourceHandler(stub)
	_, err := grantHandler.Handle(context.Background(), grantFromOrganizationAtoB())
	require.ErrorIs(t, err, domain.ErrCrossOrganizationGrant)
	require.Zero(t, stub.created)
}

func TestSameOrganizationGrantUsesExistingGrantPort(t *testing.T) {
	stub := &grantPortStub{}
	grantHandler := NewGrantResourceHandler(stub)
	org := uuid.New()
	_, err := grantHandler.Handle(context.Background(), GrantResource{
		CommandID: uuid.New(), ActorID: uuid.New(), OrganizationID: org, GranteeOrganizationID: org,
		WorkspaceID: uuid.New(), NamespaceID: uuid.New(), NodeID: uuid.New(), GranteeID: uuid.New(),
		Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
	})
	require.NoError(t, err)
	require.Equal(t, 1, stub.created)
}

func TestAnonymousGrantStaysDisabled(t *testing.T) {
	stub := &grantPortStub{}
	grantHandler := NewGrantResourceHandler(stub)
	org := uuid.New()
	_, err := grantHandler.Handle(context.Background(), GrantResource{
		CommandID: uuid.New(), ActorID: uuid.New(), OrganizationID: org, GranteeOrganizationID: org,
		WorkspaceID: uuid.New(), NamespaceID: uuid.New(), NodeID: uuid.New(), GranteeID: uuid.New(),
		Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow, Anonymous: true,
	})
	require.ErrorIs(t, err, domain.ErrAnonymousLinkDisabled)
	require.Zero(t, stub.created)
}

func grantFromOrganizationAtoB() GrantResource {
	return GrantResource{
		CommandID: uuid.New(), ActorID: uuid.New(),
		OrganizationID: uuid.New(), GranteeOrganizationID: uuid.New(),
		WorkspaceID: uuid.New(), NamespaceID: uuid.New(), NodeID: uuid.New(), GranteeID: uuid.New(),
		Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
	}
}

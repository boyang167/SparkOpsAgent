package authorization

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type workspaceAuthorityStub struct {
	organizationID uuid.UUID
	active         bool
	err            error
}

func (s workspaceAuthorityStub) ResolveWorkspaceAccess(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, bool, error) {
	return s.organizationID, s.active, s.err
}

type roleBindingStub struct {
	roleIDs []uuid.UUID
	err     error
}

func (s roleBindingStub) ListWorkspaceRoleIDs(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	return s.roleIDs, s.err
}

func TestPrincipalResolverUsesAuthoritativeWorkspaceMembershipAndRoles(t *testing.T) {
	actorID, workspaceID, organizationID, roleID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	resolver := NewPrincipalResolver(
		workspaceAuthorityStub{organizationID: organizationID, active: true},
		roleBindingStub{roleIDs: []uuid.UUID{roleID}},
	)

	principals, err := resolver.Resolve(context.Background(), ResolvePrincipalsInput{
		ActorID: actorID, WorkspaceID: workspaceID, OrganizationID: organizationID,
	})

	require.NoError(t, err)
	require.Equal(t, []repository.PrincipalRef{
		{Type: model.PrincipalUser, ID: actorID},
		{Type: model.PrincipalWorkspace, ID: workspaceID},
		{Type: model.PrincipalOrganization, ID: organizationID},
		{Type: model.PrincipalRole, ID: roleID},
	}, principals)
}

func TestPrincipalResolverExcludesWorkspaceAudienceForInactiveMember(t *testing.T) {
	actorID, workspaceID, organizationID := uuid.New(), uuid.New(), uuid.New()
	resolver := NewPrincipalResolver(
		workspaceAuthorityStub{organizationID: organizationID, active: false},
		roleBindingStub{roleIDs: []uuid.UUID{uuid.New()}},
	)

	principals, err := resolver.Resolve(context.Background(), ResolvePrincipalsInput{
		ActorID: actorID, WorkspaceID: workspaceID, OrganizationID: organizationID,
	})

	require.NoError(t, err)
	require.Equal(t, []repository.PrincipalRef{{Type: model.PrincipalUser, ID: actorID}}, principals)
}

func TestPrincipalResolverRejectsCrossOrganizationWorkspace(t *testing.T) {
	resolver := NewPrincipalResolver(
		workspaceAuthorityStub{organizationID: uuid.New(), active: true},
		roleBindingStub{},
	)

	_, err := resolver.Resolve(context.Background(), ResolvePrincipalsInput{
		ActorID: uuid.New(), WorkspaceID: uuid.New(), OrganizationID: uuid.New(),
	})

	require.ErrorIs(t, err, ErrPrincipalScopeMismatch)
}

func TestPrincipalResolverWithoutWorkspaceReturnsUserAndOrganization(t *testing.T) {
	actorID, organizationID := uuid.New(), uuid.New()
	resolver := NewPrincipalResolver(workspaceAuthorityStub{}, roleBindingStub{})

	principals, err := resolver.Resolve(context.Background(), ResolvePrincipalsInput{
		ActorID: actorID, OrganizationID: organizationID,
	})

	require.NoError(t, err)
	require.Equal(t, []repository.PrincipalRef{
		{Type: model.PrincipalUser, ID: actorID},
		{Type: model.PrincipalOrganization, ID: organizationID},
	}, principals)
}

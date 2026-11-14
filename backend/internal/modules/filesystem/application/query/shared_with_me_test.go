package query

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

func TestSharedWithMeUsesTypedPrincipalsAndForwardsStableFilters(t *testing.T) {
	orgID, workspaceID, actorID, roleID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	nodeID, grantID := uuid.New(), uuid.New()
	repo := &governanceRepositoryStub{shared: []SharedNode{{
		Node: model.Node{ID: nodeID, NamespaceID: uuid.New(), Name: "quarterly.csv"},
	}}}
	resolver := namespacePrincipalResolver{principals: []repository.PrincipalRef{
		{Type: model.PrincipalUser, ID: actorID},
		{Type: model.PrincipalWorkspace, ID: workspaceID},
		{Type: model.PrincipalRole, ID: roleID},
	}}
	handler := NewSharedWithMeHandler(repo, resolver, effectiveAuthorizerStub{decisions: map[model.ResourcePermission]resourceauth.Decision{
		model.ResourcePermissionRead:  {Allowed: true, Reason: "grant_allow", SourceGrantIDs: []uuid.UUID{grantID}},
		model.ResourcePermissionWrite: {Allowed: true, Reason: "grant_allow", SourceGrantIDs: []uuid.UUID{grantID}},
	}})

	items, next, err := handler.Handle(context.Background(), SharedWithMe{
		ActorID: actorID, OrganizationID: orgID, WorkspaceID: workspaceID,
		Query: "quarter", Cursor: uuid.NewString(), Limit: 17,
	})

	require.NoError(t, err)
	require.Empty(t, next)
	require.Len(t, items, 1)
	require.Equal(t, nodeID, items[0].Node.ID)
	require.ElementsMatch(t, []model.ResourcePermission{model.ResourcePermissionRead, model.ResourcePermissionWrite}, items[0].EffectiveActions)
	require.Equal(t, []uuid.UUID{grantID}, items[0].SourceGrantIDs)
	require.Equal(t, "quarter", repo.filter.Query)
	require.Equal(t, 17, repo.filter.Limit)
	require.Len(t, repo.filter.Principals, 3)
	require.Equal(t, model.PrincipalRole, repo.filter.Principals[2].Type)
}

func TestSharedWithMeRejectsResolverScopeMismatch(t *testing.T) {
	handler := NewSharedWithMeHandler(&governanceRepositoryStub{}, principalResolverErrorStub{err: resourceauth.ErrPrincipalScopeMismatch}, effectiveAuthorizerStub{})

	_, _, err := handler.Handle(context.Background(), SharedWithMe{
		ActorID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New(),
	})

	require.ErrorIs(t, err, resourceauth.ErrPrincipalScopeMismatch)
}

func TestSharedWithMeDropsChildAllowWhenAncestorDenyWins(t *testing.T) {
	orgID, workspaceID, actorID := uuid.New(), uuid.New(), uuid.New()
	nodeID, namespaceID := uuid.New(), uuid.New()
	denyID, allowID := uuid.New(), uuid.New()
	repo := &governanceRepositoryStub{shared: []SharedNode{{
		Node: model.Node{ID: nodeID, NamespaceID: namespaceID, Name: "hidden.txt"},
	}}}
	handler := NewSharedWithMeHandler(repo, namespacePrincipalResolver{
		principals: []repository.PrincipalRef{{Type: model.PrincipalUser, ID: actorID}},
	}, effectiveAuthorizerStub{decisions: map[model.ResourcePermission]resourceauth.Decision{
		model.ResourcePermissionRead: {
			Allowed: false, Reason: "explicit_deny", SourceGrantIDs: []uuid.UUID{denyID, allowID},
		},
	}})

	items, next, err := handler.Handle(context.Background(), SharedWithMe{
		ActorID: actorID, OrganizationID: orgID, WorkspaceID: workspaceID, Limit: 10,
	})

	require.NoError(t, err)
	require.Empty(t, next)
	require.Empty(t, items)
}

func TestSharedWithMeIncludesAllowAndDenySourcesThatShapeEffectiveActions(t *testing.T) {
	actorID, nodeID, namespaceID := uuid.New(), uuid.New(), uuid.New()
	readAllow, writeAllow, writeDeny := uuid.New(), uuid.New(), uuid.New()
	repo := &governanceRepositoryStub{shared: []SharedNode{{Node: model.Node{ID: nodeID, NamespaceID: namespaceID}}}}
	handler := NewSharedWithMeHandler(repo, namespacePrincipalResolver{
		principals: []repository.PrincipalRef{{Type: model.PrincipalUser, ID: actorID}},
	}, effectiveAuthorizerStub{decisions: map[model.ResourcePermission]resourceauth.Decision{
		model.ResourcePermissionRead: {
			Allowed: true, Reason: "grant_allow", SourceGrantIDs: []uuid.UUID{readAllow},
		},
		model.ResourcePermissionWrite: {
			Allowed: false, Reason: "explicit_deny", SourceGrantIDs: []uuid.UUID{writeAllow, writeDeny},
		},
	}})

	items, _, err := handler.Handle(context.Background(), SharedWithMe{
		ActorID: actorID, OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Limit: 10,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, []model.ResourcePermission{model.ResourcePermissionRead}, items[0].EffectiveActions)
	require.ElementsMatch(t, []uuid.UUID{readAllow, writeAllow, writeDeny}, items[0].SourceGrantIDs)
}

type principalResolverErrorStub struct {
	err error
}

func (s principalResolverErrorStub) Resolve(context.Context, resourceauth.ResolvePrincipalsInput) ([]repository.PrincipalRef, error) {
	return nil, s.err
}

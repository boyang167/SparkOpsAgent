package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

func TestListSharedNodesDeduplicatesTypedGrantsAndAppliesGlobalDeny(t *testing.T) {
	repo, _ := newTestRepository(t)
	governanceRepo := repo.(*Repository)
	ctx := context.Background()
	orgID, actorID, workspaceID, roleID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization, OrganizationID: orgID}
	require.NoError(t, repo.CreateNamespace(ctx, namespace))
	node := &model.Node{
		ID: uuid.New(), NamespaceID: namespace.ID, Type: model.NodeTypeFile,
		Name: "Quarterly Report.csv", NormalizedName: "quarterly report.csv",
		State: model.NodeStateActive, OwnerID: uuid.New(),
	}
	require.NoError(t, repo.CreateNode(ctx, node))
	readGrant, writeGrant, denyGrant, shareGrant := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, grant := range []model.ResourceGrant{
		{ID: readGrant, NodeID: node.ID, OrganizationID: orgID, PrincipalType: model.PrincipalUser, PrincipalID: actorID, GranteeID: actorID, Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow},
		{ID: writeGrant, NodeID: node.ID, OrganizationID: orgID, PrincipalType: model.PrincipalUser, PrincipalID: actorID, GranteeID: actorID, Permission: model.ResourcePermissionWrite, Effect: model.GrantEffectAllow},
		{ID: denyGrant, NodeID: node.ID, OrganizationID: orgID, PrincipalType: model.PrincipalWorkspace, PrincipalID: workspaceID, GranteeID: workspaceID, Permission: model.ResourcePermissionWrite, Effect: model.GrantEffectDeny},
		{ID: shareGrant, NodeID: node.ID, OrganizationID: orgID, PrincipalType: model.PrincipalRole, PrincipalID: roleID, GranteeID: roleID, Permission: model.ResourcePermissionShare, Effect: model.GrantEffectAllow},
	} {
		require.NoError(t, repo.CreateResourceGrant(ctx, grant))
	}
	principals := []repository.PrincipalRef{
		{Type: model.PrincipalUser, ID: actorID},
		{Type: model.PrincipalWorkspace, ID: workspaceID},
		{Type: model.PrincipalRole, ID: roleID},
	}

	items, next, err := governanceRepo.ListSharedNodes(ctx, orgID, principals, repository.SharedNodeFilter{
		NamespaceID: namespace.ID, Query: "quarter", Limit: 10,
	}, time.Now().UTC())

	require.NoError(t, err)
	require.Empty(t, next)
	require.Len(t, items, 1)
	require.Equal(t, node.ID, items[0].Node.ID)
	require.ElementsMatch(t, []model.ResourcePermission{model.ResourcePermissionRead, model.ResourcePermissionShare}, items[0].EffectiveActions)
	require.ElementsMatch(t, []uuid.UUID{readGrant, shareGrant}, items[0].SourceGrantIDs)
}

func TestListSharedNodesUsesStableNodeCursorAndOrganizationScope(t *testing.T) {
	repo, _ := newTestRepository(t)
	governanceRepo := repo.(*Repository)
	ctx := context.Background()
	orgID, actorID := uuid.New(), uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization, OrganizationID: orgID}
	foreign := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization, OrganizationID: uuid.New()}
	require.NoError(t, repo.CreateNamespace(ctx, namespace))
	require.NoError(t, repo.CreateNamespace(ctx, foreign))
	ids := []uuid.UUID{
		uuid.MustParse("10000000-0000-0000-0000-000000000001"),
		uuid.MustParse("20000000-0000-0000-0000-000000000002"),
		uuid.MustParse("30000000-0000-0000-0000-000000000003"),
	}
	for index, id := range ids {
		targetNamespace := namespace
		if index == 2 {
			targetNamespace = foreign
		}
		node := &model.Node{ID: id, NamespaceID: targetNamespace.ID, Type: model.NodeTypeFile, Name: id.String(), NormalizedName: id.String(), State: model.NodeStateActive, OwnerID: actorID}
		require.NoError(t, repo.CreateNode(ctx, node))
		require.NoError(t, repo.CreateResourceGrant(ctx, model.ResourceGrant{
			ID: uuid.New(), NodeID: id, OrganizationID: targetNamespace.OrganizationID,
			PrincipalType: model.PrincipalUser, PrincipalID: actorID, GranteeID: actorID,
			Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
		}))
	}
	principals := []repository.PrincipalRef{{Type: model.PrincipalUser, ID: actorID}}

	first, next, err := governanceRepo.ListSharedNodes(ctx, orgID, principals, repository.SharedNodeFilter{Limit: 1}, time.Now().UTC())
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, ids[0], first[0].Node.ID)
	require.Equal(t, ids[0].String(), next)

	second, next, err := governanceRepo.ListSharedNodes(ctx, orgID, principals, repository.SharedNodeFilter{Limit: 1, Cursor: next}, time.Now().UTC())
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, ids[1], second[0].Node.ID)
	require.Empty(t, next)
}

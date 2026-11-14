package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	filesystemquery "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
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

func TestListSharedNodesReturnsDenyOnlyRawCandidateForBoundedAuthorization(t *testing.T) {
	repo, _ := newTestRepository(t)
	governanceRepo := repo.(*Repository)
	ctx := context.Background()
	orgID, actorID := uuid.New(), uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization, OrganizationID: orgID}
	require.NoError(t, repo.CreateNamespace(ctx, namespace))
	node := &model.Node{
		ID: uuid.New(), NamespaceID: namespace.ID, Type: model.NodeTypeFile,
		Name: "denied.txt", NormalizedName: "denied.txt", State: model.NodeStateActive, OwnerID: actorID,
	}
	require.NoError(t, repo.CreateNode(ctx, node))
	require.NoError(t, repo.CreateResourceGrant(ctx, model.ResourceGrant{
		ID: uuid.New(), NodeID: node.ID, OrganizationID: orgID,
		PrincipalType: model.PrincipalUser, PrincipalID: actorID, GranteeID: actorID,
		Permission: model.ResourcePermissionRead, Effect: model.GrantEffectDeny,
	}))

	items, next, err := governanceRepo.ListSharedNodes(ctx, orgID,
		[]repository.PrincipalRef{{Type: model.PrincipalUser, ID: actorID}},
		repository.SharedNodeFilter{Limit: 10}, time.Now().UTC())

	require.NoError(t, err)
	require.Empty(t, next)
	require.Len(t, items, 1)
	require.Equal(t, node.ID, items[0].Node.ID)
	require.Empty(t, items[0].EffectiveActions)
}

func TestListSharedNodesProjectsAllowSourcesAndNamespaceSafePathsInBatches(t *testing.T) {
	repo, _ := newTestRepository(t)
	governanceRepo := repo.(*Repository)
	ctx := context.Background()
	orgID, actorID, userOwner := uuid.New(), uuid.New(), uuid.New()
	userNamespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeUser,
		OrganizationID: orgID, OwnerUserID: userOwner, Status: "ACTIVE",
	}
	workspaceNamespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace,
		OrganizationID: orgID, WorkspaceID: uuid.New(), Status: "ACTIVE",
	}
	require.NoError(t, repo.CreateNamespace(ctx, userNamespace))
	require.NoError(t, repo.CreateNamespace(ctx, workspaceNamespace))
	userRoot, err := model.NewRootDirectory(userNamespace.ID, userOwner, "internal-root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(ctx, userRoot))
	require.NoError(t, governanceRepo.SetNamespaceRootByID(ctx, userNamespace.ID, userRoot.ID))
	userNamedRoot, err := model.NewDirectory(userNamespace.ID, userOwner, ".root", userRoot.ID)
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(ctx, userNamedRoot))
	folder, err := model.NewDirectory(userNamespace.ID, userOwner, "项目", userNamedRoot.ID)
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(ctx, folder))
	file, err := model.NewFile(userNamespace.ID, userOwner, "方案.md", folder.ID)
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(ctx, file))

	workspaceRoot, err := model.NewRootDirectory(workspaceNamespace.ID, actorID, ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(ctx, workspaceRoot))
	require.NoError(t, governanceRepo.SetNamespaceRootByID(ctx, workspaceNamespace.ID, workspaceRoot.ID))
	workspaceFile, err := model.NewFile(workspaceNamespace.ID, actorID, "同名.md", workspaceRoot.ID)
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(ctx, workspaceFile))

	expiry := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	allowID, denyID, workspaceAllowID := uuid.New(), uuid.New(), uuid.New()
	for _, grant := range []model.ResourceGrant{
		{
			ID: allowID, NodeID: folder.ID, OrganizationID: orgID,
			PrincipalType: model.PrincipalUser, PrincipalID: actorID, GranteeID: actorID,
			Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
			InheritToChildren: true, ExpiresAt: &expiry,
		},
		{
			ID: denyID, NodeID: file.ID, OrganizationID: orgID,
			PrincipalType: model.PrincipalWorkspace, PrincipalID: workspaceNamespace.WorkspaceID,
			GranteeID: workspaceNamespace.WorkspaceID, Permission: model.ResourcePermissionWrite,
			Effect: model.GrantEffectDeny,
		},
		{
			ID: workspaceAllowID, NodeID: workspaceFile.ID, OrganizationID: orgID,
			PrincipalType: model.PrincipalUser, PrincipalID: actorID, GranteeID: actorID,
			Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
		},
	} {
		require.NoError(t, repo.CreateResourceGrant(ctx, grant))
	}

	items, _, err := governanceRepo.ListSharedNodes(
		ctx,
		orgID,
		[]repository.PrincipalRef{
			{Type: model.PrincipalUser, ID: actorID},
			{Type: model.PrincipalWorkspace, ID: workspaceNamespace.WorkspaceID},
		},
		repository.SharedNodeFilter{Limit: 10},
		time.Now().UTC(),
	)

	require.NoError(t, err)
	byID := make(map[uuid.UUID]repository.SharedNode, len(items))
	for _, item := range items {
		byID[item.Node.ID] = item
	}
	projected := byID[file.ID]
	require.Equal(t, "/.root/项目/方案.md", projected.OriginalPath)
	require.Equal(t, model.NamespaceTypeUser, projected.SourceNamespace.NamespaceType)
	require.Equal(t, orgID, projected.SourceNamespace.OrganizationID)
	require.Nil(t, projected.SourceNamespace.WorkspaceID)
	require.NotNil(t, projected.SourceNamespace.OwnerUserID)
	require.Equal(t, userOwner, *projected.SourceNamespace.OwnerUserID)
	require.Equal(t, []repository.SharedGrantSource{{
		GrantID: allowID, PrincipalType: model.PrincipalUser,
		PrincipalID: actorID, Permission: model.ResourcePermissionRead,
		Effect: model.GrantEffectAllow, ExpiresAt: &expiry,
	}}, projected.Sources)
	require.NotContains(t, projected.SourceGrantIDs, denyID)
	require.Equal(t, "/同名.md", byID[workspaceFile.ID].OriginalPath)
	require.Equal(t, orgID, byID[workspaceFile.ID].SourceNamespace.OrganizationID)
	require.NotNil(t, byID[workspaceFile.ID].SourceNamespace.WorkspaceID)
	require.Equal(t, workspaceNamespace.WorkspaceID, *byID[workspaceFile.ID].SourceNamespace.WorkspaceID)
	require.NotContains(t, projected.OriginalPath, "internal-root")
}

func TestSharedWithMeProjectsAuthoritativeAgentAndServiceAccountPrincipals(t *testing.T) {
	repo, _ := newTestRepository(t)
	governanceRepo := repo.(*Repository)
	ctx := context.Background()
	orgID, actorID := uuid.New(), uuid.New()
	namespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization,
		OrganizationID: orgID, Status: "ACTIVE",
	}
	require.NoError(t, repo.CreateNamespace(ctx, namespace))
	root, err := model.NewRootDirectory(namespace.ID, actorID, ".root")
	require.NoError(t, err)
	require.NoError(t, repo.CreateNode(ctx, root))
	require.NoError(t, governanceRepo.SetNamespaceRootByID(ctx, namespace.ID, root.ID))

	agentID, serviceAccountID := uuid.New(), uuid.New()
	principals := []repository.PrincipalRef{
		{Type: model.PrincipalUser, ID: actorID},
		{Type: model.PrincipalAgent, ID: agentID},
		{Type: model.PrincipalServiceAccount, ID: serviceAccountID},
	}
	resolver := sharedPrincipalResolverStub{principals: principals}
	grantByNode := make(map[uuid.UUID]uuid.UUID)
	for index, principal := range principals[1:] {
		node, nodeErr := model.NewFile(namespace.ID, actorID, []string{"agent.md", "service.md"}[index], root.ID)
		require.NoError(t, nodeErr)
		require.NoError(t, repo.CreateNode(ctx, node))
		grantID := uuid.New()
		grantByNode[node.ID] = grantID
		require.NoError(t, repo.CreateResourceGrant(ctx, model.ResourceGrant{
			ID: grantID, NodeID: node.ID, OrganizationID: orgID,
			PrincipalType: principal.Type, PrincipalID: principal.ID, GranteeID: principal.ID,
			Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
		}))
	}
	authorizer := resourceauth.NewService(governanceRepo, resolver, sharedAuthorityStub{})
	codec, err := filesystemquery.NewSharedCursorCodec("shared-principal-chain-test", nil)
	require.NoError(t, err)
	handler := filesystemquery.NewSharedWithMeHandler(governanceRepo, resolver, authorizer, codec)

	items, _, err := handler.Handle(ctx, filesystemquery.SharedWithMe{
		ActorID: actorID, OrganizationID: orgID, Limit: 10,
	})

	require.NoError(t, err)
	require.Len(t, items, 2)
	for _, item := range items {
		require.Equal(t, []uuid.UUID{grantByNode[item.Node.ID]}, item.SourceGrantIDs)
		require.Len(t, item.Sources, 1)
		require.Contains(t,
			[]model.PrincipalType{model.PrincipalAgent, model.PrincipalServiceAccount},
			item.Sources[0].PrincipalType,
		)
	}
	serviceItems, _, err := handler.Handle(ctx, filesystemquery.SharedWithMe{
		ActorID: actorID, OrganizationID: orgID,
		SourceType: model.PrincipalServiceAccount, Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, serviceItems, 1)
	require.Equal(t, model.PrincipalServiceAccount, serviceItems[0].Sources[0].PrincipalType)
}

type sharedPrincipalResolverStub struct {
	principals []repository.PrincipalRef
}

func (s sharedPrincipalResolverStub) Resolve(context.Context, resourceauth.ResolvePrincipalsInput) ([]repository.PrincipalRef, error) {
	return s.principals, nil
}

type sharedAuthorityStub struct{}

func (sharedAuthorityStub) HasWorkspaceFileAdmin(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (sharedAuthorityStub) HasOrganizationFileAdmin(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

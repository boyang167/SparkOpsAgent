package query

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

func TestSharedWithMeUsesTypedPrincipalsAndForwardsStableFilters(t *testing.T) {
	orgID, workspaceID, actorID, roleID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	nodeID, grantID := uuid.New(), uuid.New()
	repo := &governanceRepositoryStub{shared: []SharedNode{{
		Node: model.Node{ID: nodeID, NamespaceID: uuid.New(), Name: "quarterly.csv"},
		Sources: []repository.SharedGrantSource{{
			GrantID: grantID, PrincipalType: model.PrincipalUser, PrincipalID: actorID,
		}},
	}}}
	resolver := namespacePrincipalResolver{principals: []repository.PrincipalRef{
		{Type: model.PrincipalUser, ID: actorID},
		{Type: model.PrincipalWorkspace, ID: workspaceID},
		{Type: model.PrincipalRole, ID: roleID},
	}}
	cursorCodec := testSharedCursorCodec(t)
	handler := NewSharedWithMeHandler(repo, resolver, effectiveAuthorizerStub{decisions: map[model.ResourcePermission]resourceauth.Decision{
		model.ResourcePermissionRead:  {Allowed: true, Reason: "grant_allow", SourceGrantIDs: []uuid.UUID{grantID}},
		model.ResourcePermissionWrite: {Allowed: true, Reason: "grant_allow", SourceGrantIDs: []uuid.UUID{grantID}},
	}}, cursorCodec)

	rawCursor := uuid.NewString()
	cursor, err := cursorCodec.Encode(rawCursor, SharedCursorContext{
		OrganizationID: orgID, ActorID: actorID, WorkspaceID: workspaceID, Query: "quarter",
	})
	require.NoError(t, err)
	items, next, err := handler.Handle(context.Background(), SharedWithMe{
		ActorID: actorID, OrganizationID: orgID, WorkspaceID: workspaceID,
		Query: "quarter", Cursor: cursor, Limit: 17,
	})

	require.NoError(t, err)
	require.Empty(t, next)
	require.Len(t, items, 1)
	require.Equal(t, nodeID, items[0].Node.ID)
	require.ElementsMatch(t, []model.ResourcePermission{model.ResourcePermissionRead, model.ResourcePermissionWrite}, items[0].EffectiveActions)
	require.Equal(t, []uuid.UUID{grantID}, items[0].SourceGrantIDs)
	require.Equal(t, "quarter", repo.filter.Query)
	require.Equal(t, SharedCandidateChunkSize, repo.filter.Limit)
	require.Equal(t, rawCursor, repo.filter.Cursor)
	require.Len(t, repo.filter.Principals, 3)
	require.Equal(t, model.PrincipalRole, repo.filter.Principals[2].Type)
}

type projectionWorkspaceAuthority struct {
	organizationID uuid.UUID
	active         bool
}

func (a projectionWorkspaceAuthority) ResolveWorkspaceAccess(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, bool, error) {
	return a.organizationID, a.active, nil
}

type projectionRoleReader struct{}

func (projectionRoleReader) ListWorkspaceRoleIDs(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

type projectionAuthority struct{}

func (projectionAuthority) HasWorkspaceFileAdmin(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (projectionAuthority) HasOrganizationFileAdmin(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

type organizationGrantProjectionRepo struct {
	namespace *model.Namespace
	node      *model.Node
	grant     model.ResourceGrant
}

func (r organizationGrantProjectionRepo) GetNamespace(context.Context, uuid.UUID) (*model.Namespace, error) {
	return r.namespace, nil
}

func (r organizationGrantProjectionRepo) GetNode(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error) {
	return r.node, nil
}

func (r organizationGrantProjectionRepo) GetNodeIncludingTrashed(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error) {
	return r.node, nil
}

func (r organizationGrantProjectionRepo) ListResourceGrants(
	_ context.Context,
	_ []uuid.UUID,
	principals []repository.PrincipalRef,
	permission model.ResourcePermission,
	_ time.Time,
) ([]model.ResourceGrant, error) {
	if permission == r.grant.Permission && containsProjectionPrincipal(principals, r.grant.PrincipalType, r.grant.PrincipalID) {
		return []model.ResourceGrant{r.grant}, nil
	}
	return nil, nil
}

func (r organizationGrantProjectionRepo) GetNodesWithAncestors(context.Context, uuid.UUID, []uuid.UUID) ([]model.Node, error) {
	return []model.Node{*r.node}, nil
}

func (r organizationGrantProjectionRepo) ListResourceGrantsForActions(
	_ context.Context,
	_ []uuid.UUID,
	principals []repository.PrincipalRef,
	permissions []model.ResourcePermission,
	_ time.Time,
) ([]model.ResourceGrant, error) {
	for _, permission := range permissions {
		if permission == r.grant.Permission && containsProjectionPrincipal(principals, r.grant.PrincipalType, r.grant.PrincipalID) {
			return []model.ResourceGrant{r.grant}, nil
		}
	}
	return nil, nil
}

func (r organizationGrantProjectionRepo) ListSharedNodes(
	_ context.Context,
	_ uuid.UUID,
	principals []repository.PrincipalRef,
	_ SharedWithMeFilter,
	_ time.Time,
) ([]SharedNode, string, error) {
	if !containsProjectionPrincipal(principals, r.grant.PrincipalType, r.grant.PrincipalID) {
		return nil, "", nil
	}
	return []SharedNode{{
		Node: *r.node,
		Sources: []repository.SharedGrantSource{{
			GrantID: r.grant.ID, PrincipalType: r.grant.PrincipalType,
			PrincipalID: r.grant.PrincipalID, Permission: r.grant.Permission,
			Effect: r.grant.Effect,
		}},
	}}, "", nil
}

func containsProjectionPrincipal(principals []repository.PrincipalRef, principalType model.PrincipalType, principalID uuid.UUID) bool {
	for _, principal := range principals {
		if principal.Type == principalType && principal.ID == principalID {
			return true
		}
	}
	return false
}

func TestSharedWithMeOrganizationGrantRequiresActiveWorkspaceMembership(t *testing.T) {
	organizationID, workspaceID, actorID := uuid.New(), uuid.New(), uuid.New()
	namespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization,
		OrganizationID: organizationID, PolicyVersion: 1,
	}
	node := &model.Node{ID: uuid.New(), NamespaceID: namespace.ID, State: model.NodeStateActive}
	grant := model.ResourceGrant{
		ID: uuid.New(), NodeID: node.ID, OrganizationID: organizationID,
		PrincipalType: model.PrincipalOrganization, PrincipalID: organizationID,
		Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
	}
	repo := organizationGrantProjectionRepo{namespace: namespace, node: node, grant: grant}

	tests := []struct {
		name          string
		workspaceID   uuid.UUID
		resolvedOrgID uuid.UUID
		active        bool
		wantItems     int
		wantScopeErr  bool
	}{
		{name: "organization only context", resolvedOrgID: organizationID},
		{name: "active workspace membership", workspaceID: workspaceID, resolvedOrgID: organizationID, active: true, wantItems: 1},
		{name: "inactive workspace membership", workspaceID: workspaceID, resolvedOrgID: organizationID},
		{name: "cross organization workspace", workspaceID: workspaceID, resolvedOrgID: uuid.New(), active: true, wantScopeErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := resourceauth.NewPrincipalResolver(
				projectionWorkspaceAuthority{organizationID: tt.resolvedOrgID, active: tt.active},
				projectionRoleReader{},
			)
			authorizer := resourceauth.NewService(repo, resolver, projectionAuthority{})
			handler := NewSharedWithMeHandler(repo, resolver, authorizer, testSharedCursorCodec(t))

			items, _, err := handler.Handle(context.Background(), SharedWithMe{
				ActorID: actorID, OrganizationID: organizationID,
				WorkspaceID: tt.workspaceID, Limit: 10,
			})

			if tt.wantScopeErr {
				require.ErrorIs(t, err, resourceauth.ErrPrincipalScopeMismatch)
				require.Empty(t, items)
				return
			}
			require.NoError(t, err)
			require.Len(t, items, tt.wantItems)
		})
	}
}

func TestSharedWithMeReadDoesNotInvokePlacementOrCreateReplica(t *testing.T) {
	actorID := uuid.New()
	repo := &sharedReadSideEffectProbe{governanceRepositoryStub: governanceRepositoryStub{
		shared: []SharedNode{{Node: model.Node{ID: uuid.New(), NamespaceID: uuid.New()}}},
	}}
	codec := testSharedCursorCodec(t)
	handler := NewSharedWithMeHandler(repo, namespacePrincipalResolver{
		principals: []repository.PrincipalRef{{Type: model.PrincipalUser, ID: actorID}},
	}, effectiveAuthorizerStub{decisions: map[model.ResourcePermission]resourceauth.Decision{
		model.ResourcePermissionRead: {Allowed: true},
	}}, codec)

	items, _, err := handler.Handle(context.Background(), SharedWithMe{
		ActorID: actorID, OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Limit: 10,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Zero(t, repo.placementCalls)
	require.Zero(t, repo.replicaCreates)
}

type sharedReadSideEffectProbe struct {
	governanceRepositoryStub
	placementCalls int
	replicaCreates int
}

func (r *sharedReadSideEffectProbe) Place(context.Context, any) {
	r.placementCalls++
}

func (r *sharedReadSideEffectProbe) InsertReplica(context.Context, *model.StorageReplica) error {
	r.replicaCreates++
	return nil
}

func TestSharedWithMeRejectsResolverScopeMismatch(t *testing.T) {
	handler := NewSharedWithMeHandler(&governanceRepositoryStub{}, principalResolverErrorStub{err: resourceauth.ErrPrincipalScopeMismatch}, effectiveAuthorizerStub{}, testSharedCursorCodec(t))

	_, _, err := handler.Handle(context.Background(), SharedWithMe{
		ActorID: uuid.New(), OrganizationID: uuid.New(), WorkspaceID: uuid.New(),
	})

	require.ErrorIs(t, err, resourceauth.ErrPrincipalScopeMismatch)
}

func TestSharedWithMeRejectsCursorReplayWithoutRestartingScan(t *testing.T) {
	orgID, actorID := uuid.New(), uuid.New()
	repo := &governanceRepositoryStub{}
	codec := testSharedCursorCodec(t)
	cursor, err := codec.Encode(uuid.NewString(), SharedCursorContext{
		OrganizationID: orgID, ActorID: actorID, Query: "quarterly",
	})
	require.NoError(t, err)
	handler := NewSharedWithMeHandler(repo, namespacePrincipalResolver{
		principals: []repository.PrincipalRef{{Type: model.PrincipalUser, ID: actorID}},
	}, effectiveAuthorizerStub{}, codec)

	_, _, err = handler.Handle(context.Background(), SharedWithMe{
		ActorID: actorID, OrganizationID: orgID, Query: "annual", Cursor: cursor,
	})

	require.ErrorIs(t, err, domain.ErrInvalidGovernanceCursor)
	require.Zero(t, repo.listCalls)
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
	}}, testSharedCursorCodec(t))

	items, next, err := handler.Handle(context.Background(), SharedWithMe{
		ActorID: actorID, OrganizationID: orgID, WorkspaceID: workspaceID, Limit: 10,
	})

	require.NoError(t, err)
	require.Empty(t, next)
	require.Empty(t, items)
}

func TestSharedWithMeOnlyExposesSourcesForEffectiveAllows(t *testing.T) {
	actorID, nodeID, namespaceID := uuid.New(), uuid.New(), uuid.New()
	readAllow, writeAllow, writeDeny := uuid.New(), uuid.New(), uuid.New()
	repo := &governanceRepositoryStub{shared: []SharedNode{{
		Node: model.Node{ID: nodeID, NamespaceID: namespaceID},
		Sources: []repository.SharedGrantSource{
			{GrantID: readAllow, PrincipalType: model.PrincipalUser, PrincipalID: actorID},
			{GrantID: writeAllow, PrincipalType: model.PrincipalUser, PrincipalID: actorID},
		},
	}}}
	handler := NewSharedWithMeHandler(repo, namespacePrincipalResolver{
		principals: []repository.PrincipalRef{{Type: model.PrincipalUser, ID: actorID}},
	}, effectiveAuthorizerStub{decisions: map[model.ResourcePermission]resourceauth.Decision{
		model.ResourcePermissionRead: {
			Allowed: true, Reason: "grant_allow", SourceGrantIDs: []uuid.UUID{readAllow},
		},
		model.ResourcePermissionWrite: {
			Allowed: false, Reason: "explicit_deny", SourceGrantIDs: []uuid.UUID{writeAllow, writeDeny},
		},
	}}, testSharedCursorCodec(t))

	items, _, err := handler.Handle(context.Background(), SharedWithMe{
		ActorID: actorID, OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Limit: 10,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, []model.ResourcePermission{model.ResourcePermissionRead}, items[0].EffectiveActions)
	require.Equal(t, []uuid.UUID{readAllow}, items[0].SourceGrantIDs)
	require.Equal(t, []repository.SharedGrantSource{{
		GrantID: readAllow, PrincipalType: model.PrincipalUser, PrincipalID: actorID,
	}}, items[0].Sources)
}

func TestSharedWithMeFiltersByEffectiveSourceTypeAndBindsCursorContext(t *testing.T) {
	actorID, organizationID, namespaceID := uuid.New(), uuid.New(), uuid.New()
	userNode, agentNode := uuid.New(), uuid.New()
	userGrant, agentGrant := uuid.New(), uuid.New()
	repo := &governanceRepositoryStub{shared: []SharedNode{
		{
			Node: model.Node{ID: userNode, NamespaceID: namespaceID},
			Sources: []repository.SharedGrantSource{{
				GrantID: userGrant, PrincipalType: model.PrincipalUser, PrincipalID: actorID,
			}},
		},
		{
			Node: model.Node{ID: agentNode, NamespaceID: namespaceID},
			Sources: []repository.SharedGrantSource{{
				GrantID: agentGrant, PrincipalType: model.PrincipalAgent, PrincipalID: uuid.New(),
			}},
		},
	}}
	codec := testSharedCursorCodec(t)
	handler := NewSharedWithMeHandler(repo, namespacePrincipalResolver{
		principals: []repository.PrincipalRef{
			{Type: model.PrincipalUser, ID: actorID},
			{Type: model.PrincipalAgent, ID: repo.shared[1].Sources[0].PrincipalID},
		},
	}, effectiveAuthorizerByNodeStub{decisions: map[uuid.UUID]map[model.ResourcePermission]resourceauth.Decision{
		userNode: {
			model.ResourcePermissionRead: {Allowed: true, SourceGrantIDs: []uuid.UUID{userGrant}},
		},
		agentNode: {
			model.ResourcePermissionRead: {Allowed: true, SourceGrantIDs: []uuid.UUID{agentGrant}},
		},
	}}, codec)

	items, _, err := handler.Handle(context.Background(), SharedWithMe{
		ActorID: actorID, OrganizationID: organizationID, NamespaceID: namespaceID,
		SourceType: model.PrincipalUser, Limit: 1,
	})

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, userNode, items[0].Node.ID)
	cursor, err := codec.Encode(userNode.String(), SharedCursorContext{
		OrganizationID: organizationID, ActorID: actorID, NamespaceID: namespaceID,
		SourceType: model.PrincipalUser,
	})
	require.NoError(t, err)
	_, _, err = handler.Handle(context.Background(), SharedWithMe{
		ActorID: actorID, OrganizationID: organizationID, NamespaceID: namespaceID,
		SourceType: model.PrincipalAgent, Limit: 1, Cursor: cursor,
	})
	require.ErrorIs(t, err, domain.ErrInvalidGovernanceCursor)
}

type principalResolverErrorStub struct {
	err error
}

type countingBatchAuthorizer struct {
	visibleIDs map[uuid.UUID]bool
	calls      int
	maxBatch   int
}

type effectiveAuthorizerByNodeStub struct {
	decisions map[uuid.UUID]map[model.ResourcePermission]resourceauth.Decision
}

func (a effectiveAuthorizerByNodeStub) Authorize(context.Context, resourceauth.Request) resourceauth.Decision {
	return resourceauth.Decision{}
}

func (a effectiveAuthorizerByNodeStub) AuthorizeMany(_ context.Context, request resourceauth.BatchRequest) resourceauth.BatchDecisions {
	out := make(resourceauth.BatchDecisions, len(request.NodeIDs))
	for _, nodeID := range request.NodeIDs {
		out[nodeID] = a.decisions[nodeID]
	}
	return out
}

func (a *countingBatchAuthorizer) Authorize(context.Context, resourceauth.Request) resourceauth.Decision {
	return resourceauth.Decision{}
}

func (a *countingBatchAuthorizer) AuthorizeMany(_ context.Context, request resourceauth.BatchRequest) resourceauth.BatchDecisions {
	a.calls++
	if len(request.NodeIDs) > a.maxBatch {
		a.maxBatch = len(request.NodeIDs)
	}
	out := make(resourceauth.BatchDecisions, len(request.NodeIDs))
	for _, nodeID := range request.NodeIDs {
		out[nodeID] = map[model.ResourcePermission]resourceauth.Decision{}
		for _, permission := range request.Permissions {
			out[nodeID][permission] = resourceauth.Decision{
				Allowed: a.visibleIDs[nodeID] && permission == model.ResourcePermissionRead,
			}
		}
	}
	return out
}

func TestSharedWithMeUsesBoundedBatchEvaluationAndOpaqueContinuation(t *testing.T) {
	actorID, organizationID, namespaceID := uuid.New(), uuid.New(), uuid.New()
	candidates := make([]SharedNode, 300)
	for index := range candidates {
		candidates[index].Node = model.Node{
			ID:          uuid.MustParse(fmt.Sprintf("%08x-0000-0000-0000-000000000000", index+1)),
			NamespaceID: namespaceID,
		}
	}
	repo := &governanceRepositoryStub{}
	repo.listFn = func(filter SharedWithMeFilter) ([]SharedNode, string, error) {
		start := 0
		if filter.Cursor != "" {
			for index := range candidates {
				if candidates[index].Node.ID.String() == filter.Cursor {
					start = index + 1
					break
				}
			}
		}
		end := start + filter.Limit
		if end > len(candidates) {
			end = len(candidates)
		}
		next := ""
		if end < len(candidates) {
			next = candidates[end-1].Node.ID.String()
		}
		return candidates[start:end], next, nil
	}
	authorizer := &countingBatchAuthorizer{visibleIDs: map[uuid.UUID]bool{
		candidates[256].Node.ID: true,
		candidates[257].Node.ID: true,
	}}
	handler := NewSharedWithMeHandler(repo, namespacePrincipalResolver{
		principals: []repository.PrincipalRef{{Type: model.PrincipalUser, ID: actorID}},
	}, authorizer, testSharedCursorCodec(t))

	first, cursor, err := handler.Handle(context.Background(), SharedWithMe{
		ActorID: actorID, OrganizationID: organizationID, NamespaceID: namespaceID, Limit: 1,
	})

	require.NoError(t, err)
	require.Empty(t, first)
	require.NotEmpty(t, cursor)
	_, parseErr := uuid.Parse(cursor)
	require.Error(t, parseErr, "continuation must not reveal the scanned node UUID")
	require.Equal(t, 4, authorizer.calls)
	require.LessOrEqual(t, authorizer.maxBatch, SharedCandidateChunkSize)

	second, next, err := handler.Handle(context.Background(), SharedWithMe{
		ActorID: actorID, OrganizationID: organizationID, NamespaceID: namespaceID,
		Limit: 1, Cursor: cursor,
	})
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, candidates[256].Node.ID, second[0].Node.ID)
	require.NotEmpty(t, next)

	third, finalCursor, err := handler.Handle(context.Background(), SharedWithMe{
		ActorID: actorID, OrganizationID: organizationID, NamespaceID: namespaceID,
		Limit: 1, Cursor: next,
	})
	require.NoError(t, err)
	require.Len(t, third, 1)
	require.Equal(t, candidates[257].Node.ID, third[0].Node.ID)
	require.NotEqual(t, second[0].Node.ID, third[0].Node.ID)
	require.Empty(t, finalCursor)
	require.LessOrEqual(t, repo.listCalls, 9)
}

func testSharedCursorCodec(t *testing.T) *SharedCursorCodec {
	t.Helper()
	codec, err := NewSharedCursorCodec("shared-cursor-test-secret", nil)
	require.NoError(t, err)
	return codec
}

func (s principalResolverErrorStub) Resolve(context.Context, resourceauth.ResolvePrincipalsInput) ([]repository.PrincipalRef, error) {
	return nil, s.err
}

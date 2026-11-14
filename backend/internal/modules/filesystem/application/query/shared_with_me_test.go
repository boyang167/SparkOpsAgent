package query

import (
	"context"
	"fmt"
	"testing"

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
	}}, testSharedCursorCodec(t))

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

type countingBatchAuthorizer struct {
	visibleIDs map[uuid.UUID]bool
	calls      int
	maxBatch   int
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

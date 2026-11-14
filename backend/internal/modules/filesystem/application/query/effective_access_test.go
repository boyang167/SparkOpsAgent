package query

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type effectiveAuthorizerStub struct {
	decisions map[model.ResourcePermission]resourceauth.Decision
}

func (s effectiveAuthorizerStub) Authorize(_ context.Context, request resourceauth.Request) resourceauth.Decision {
	return s.decisions[request.Permission]
}

func TestEffectiveAccessMergesAllowedActionsAndGrantSources(t *testing.T) {
	orgID, workspaceID, actorID, namespaceID, nodeID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	readGrant, writeGrant := uuid.New(), uuid.New()
	cacheKey := resourceauth.AuthorizationCacheKey{
		OrganizationID: orgID, WorkspaceID: workspaceID, ActorID: actorID,
		NodeID: nodeID, Action: model.ResourcePermissionRead, PolicyVersion: 9,
	}
	handler := NewEffectiveAccessHandler(effectiveAuthorizerStub{decisions: map[model.ResourcePermission]resourceauth.Decision{
		model.ResourcePermissionRead: {
			Allowed: true, Reason: "grant_allow", PolicyVersion: 9, CacheKey: cacheKey,
			SourceGrantIDs: []uuid.UUID{readGrant},
		},
		model.ResourcePermissionWrite: {
			Allowed: true, Reason: "grant_allow", PolicyVersion: 9,
			SourceGrantIDs: []uuid.UUID{readGrant, writeGrant},
		},
		model.ResourcePermissionDelete: {Reason: "explicit_deny", PolicyVersion: 9},
		model.ResourcePermissionShare:  {Reason: "default_deny", PolicyVersion: 9},
		model.ResourcePermissionAdmin:  {Reason: "default_deny", PolicyVersion: 9},
	}})

	got := handler.Handle(context.Background(), EffectiveAccess{
		ActorID: actorID, WorkspaceID: workspaceID, NamespaceID: namespaceID, NodeID: nodeID,
	})

	require.Equal(t, "ALLOW", got.Decision)
	require.ElementsMatch(t, []model.ResourcePermission{model.ResourcePermissionRead, model.ResourcePermissionWrite}, got.EffectiveActions)
	require.Equal(t, "grant_allow", got.ReasonCode)
	require.ElementsMatch(t, []uuid.UUID{readGrant, writeGrant}, got.SourceGrantIDs)
	require.Equal(t, int64(9), got.PolicyVersion)
	require.Equal(t, cacheKey, got.CacheKey)
}

func TestEffectiveAccessDoesNotTurnAdminCapabilityIntoResourceAllow(t *testing.T) {
	denyGrant := uuid.New()
	handler := NewEffectiveAccessHandler(effectiveAuthorizerStub{decisions: map[model.ResourcePermission]resourceauth.Decision{
		model.ResourcePermissionRead:   {Reason: "explicit_deny", PolicyVersion: 4, SourceGrantIDs: []uuid.UUID{denyGrant}},
		model.ResourcePermissionWrite:  {Reason: "default_deny", PolicyVersion: 4},
		model.ResourcePermissionDelete: {Reason: "default_deny", PolicyVersion: 4},
		model.ResourcePermissionShare:  {Reason: "default_deny", PolicyVersion: 4},
		model.ResourcePermissionAdmin:  {Reason: "default_deny", PolicyVersion: 4},
	}})

	got := handler.Handle(context.Background(), EffectiveAccess{
		ActorID: uuid.New(), WorkspaceID: uuid.New(), NamespaceID: uuid.New(), NodeID: uuid.New(),
	})

	require.Equal(t, "DENY", got.Decision)
	require.Empty(t, got.EffectiveActions)
	require.Equal(t, "explicit_deny", got.ReasonCode)
	require.Equal(t, []uuid.UUID{denyGrant}, got.SourceGrantIDs)
}

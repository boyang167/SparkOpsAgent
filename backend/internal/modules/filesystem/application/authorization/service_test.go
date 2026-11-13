package authorization

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type grantRepositoryStub struct {
	nodes  map[uuid.UUID]*model.Node
	grants map[uuid.UUID][]model.ResourceGrant
}

func (r grantRepositoryStub) GetNode(_ context.Context, namespaceID, nodeID uuid.UUID) (*model.Node, error) {
	node := r.nodes[nodeID]
	if node == nil || node.NamespaceID != namespaceID {
		return nil, domain.ErrNodeNotFound
	}
	return node, nil
}

func (r grantRepositoryStub) ListResourceGrants(_ context.Context, nodeID, granteeID uuid.UUID, permission model.ResourcePermission, at time.Time) ([]model.ResourceGrant, error) {
	var result []model.ResourceGrant
	for _, grant := range r.grants[nodeID] {
		if grant.GranteeID == granteeID && grant.Permission == permission && (grant.ExpiresAt == nil || grant.ExpiresAt.After(at)) {
			result = append(result, grant)
		}
	}
	return result, nil
}

func TestAuthorizeInheritsAllowFromAncestor(t *testing.T) {
	namespaceID, owner, actor := uuid.New(), uuid.New(), uuid.New()
	root := node(namespaceID, uuid.Nil, owner)
	child := node(namespaceID, root.ID, owner)
	service := NewService(grantRepositoryStub{
		nodes: map[uuid.UUID]*model.Node{root.ID: root, child.ID: child},
		grants: map[uuid.UUID][]model.ResourceGrant{
			root.ID: {{NodeID: root.ID, GranteeID: actor, Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow}},
		},
	}, nil)

	decision := service.Authorize(context.Background(), Request{ActorID: actor, NamespaceID: namespaceID, NodeID: child.ID, Permission: model.ResourcePermissionRead})

	require.True(t, decision.Allowed)
	require.Equal(t, "grant_allow", decision.Reason)
}

func TestAuthorizeExplicitDenyWinsAcrossAncestors(t *testing.T) {
	namespaceID, owner, actor := uuid.New(), uuid.New(), uuid.New()
	root := node(namespaceID, uuid.Nil, owner)
	child := node(namespaceID, root.ID, owner)
	service := NewService(grantRepositoryStub{
		nodes: map[uuid.UUID]*model.Node{root.ID: root, child.ID: child},
		grants: map[uuid.UUID][]model.ResourceGrant{
			root.ID:  {{NodeID: root.ID, GranteeID: actor, Permission: model.ResourcePermissionWrite, Effect: model.GrantEffectAllow}},
			child.ID: {{NodeID: child.ID, GranteeID: actor, Permission: model.ResourcePermissionWrite, Effect: model.GrantEffectDeny}},
		},
	}, nil)

	decision := service.Authorize(context.Background(), Request{ActorID: actor, NamespaceID: namespaceID, NodeID: child.ID, Permission: model.ResourcePermissionWrite})

	require.False(t, decision.Allowed)
	require.Equal(t, "explicit_deny", decision.Reason)
}

func TestAuthorizeOwnerAdminDefaultDenyAndNamespaceBoundary(t *testing.T) {
	namespaceID, owner, stranger := uuid.New(), uuid.New(), uuid.New()
	target := node(namespaceID, uuid.Nil, owner)
	service := NewService(grantRepositoryStub{nodes: map[uuid.UUID]*model.Node{target.ID: target}}, func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
		return true, nil
	})

	require.Equal(t, "owner", service.Authorize(context.Background(), Request{ActorID: owner, NamespaceID: namespaceID, NodeID: target.ID, Permission: model.ResourcePermissionWrite}).Reason)
	require.Equal(t, "admin_bypass", service.Authorize(context.Background(), Request{ActorID: stranger, NamespaceID: namespaceID, NodeID: target.ID, Permission: model.ResourcePermissionAdmin}).Reason)

	withoutAdmin := NewService(grantRepositoryStub{nodes: map[uuid.UUID]*model.Node{target.ID: target}}, nil)
	require.Equal(t, "default_deny", withoutAdmin.Authorize(context.Background(), Request{ActorID: stranger, NamespaceID: namespaceID, NodeID: target.ID, Permission: model.ResourcePermissionRead}).Reason)
	require.Equal(t, "resource_not_found", withoutAdmin.Authorize(context.Background(), Request{ActorID: owner, NamespaceID: uuid.New(), NodeID: target.ID, Permission: model.ResourcePermissionRead}).Reason)
}

func node(namespaceID, parentID, owner uuid.UUID) *model.Node {
	parent := &parentID
	if parentID == uuid.Nil {
		parent = nil
	}
	return &model.Node{ID: uuid.New(), NamespaceID: namespaceID, ParentID: parent, Type: model.NodeTypeDirectory, State: model.NodeStateActive, OwnerID: owner}
}

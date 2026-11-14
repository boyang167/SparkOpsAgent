package authorization

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type grantRepositoryStub struct {
	nodes      map[uuid.UUID]*model.Node
	namespaces map[uuid.UUID]*model.Namespace
	grants     map[uuid.UUID][]model.ResourceGrant
}

func (r grantRepositoryStub) GetNamespace(_ context.Context, namespaceID uuid.UUID) (*model.Namespace, error) {
	namespace := r.namespaces[namespaceID]
	if namespace == nil {
		return nil, domain.ErrNamespaceNotFound
	}
	return namespace, nil
}

func (r grantRepositoryStub) GetNode(_ context.Context, namespaceID, nodeID uuid.UUID) (*model.Node, error) {
	node := r.nodes[nodeID]
	if node == nil || node.NamespaceID != namespaceID || node.State != model.NodeStateActive {
		return nil, domain.ErrNodeNotFound
	}
	return node, nil
}

func (r grantRepositoryStub) GetNodeIncludingTrashed(_ context.Context, namespaceID, nodeID uuid.UUID) (*model.Node, error) {
	node := r.nodes[nodeID]
	if node == nil || node.NamespaceID != namespaceID {
		return nil, domain.ErrNodeNotFound
	}
	return node, nil
}

func TestAuthorizeDeleteReplayTraversesTrashedAncestors(t *testing.T) {
	namespaceID, owner := uuid.New(), uuid.New()
	root := node(namespaceID, uuid.Nil, owner)
	root.State = model.NodeStateTrashed
	child := node(namespaceID, root.ID, owner)
	child.State = model.NodeStateTrashed
	service := NewService(grantRepositoryStub{
		nodes:      map[uuid.UUID]*model.Node{root.ID: root, child.ID: child},
		namespaces: map[uuid.UUID]*model.Namespace{namespaceID: userNamespace(namespaceID, owner)},
	}, nil, nil)

	require.True(t, service.Authorize(context.Background(), Request{
		ActorID: owner, NamespaceID: namespaceID, NodeID: child.ID, Permission: model.ResourcePermissionDelete,
	}).Allowed)
	require.Equal(t, "resource_not_found", service.Authorize(context.Background(), Request{
		ActorID: owner, NamespaceID: namespaceID, NodeID: child.ID, Permission: model.ResourcePermissionRead,
	}).Reason)
}

func TestAuthorizeAllowsExplicitUserGrantFromAnotherWorkspaceNamespace(t *testing.T) {
	organizationID, sourceWorkspaceID, currentWorkspaceID := uuid.New(), uuid.New(), uuid.New()
	actorID, namespaceID, nodeID, grantID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	namespace := &model.Namespace{
		ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace,
		OrganizationID: organizationID, WorkspaceID: sourceWorkspaceID, PolicyVersion: 1,
	}
	target := &model.Node{ID: nodeID, NamespaceID: namespaceID, State: model.NodeStateActive}
	service := NewService(grantRepositoryStub{
		nodes:      map[uuid.UUID]*model.Node{nodeID: target},
		namespaces: map[uuid.UUID]*model.Namespace{namespaceID: namespace},
		grants: map[uuid.UUID][]model.ResourceGrant{
			nodeID: {{
				ID: grantID, NodeID: nodeID, OrganizationID: organizationID,
				PrincipalType: model.PrincipalUser, PrincipalID: actorID, GranteeID: actorID,
				Permission: model.ResourcePermissionShare, Effect: model.GrantEffectAllow,
			}},
		},
	}, principalResolverStub{principals: []repository.PrincipalRef{
		{Type: model.PrincipalUser, ID: actorID},
		{Type: model.PrincipalWorkspace, ID: currentWorkspaceID},
	}}, authorityStub{})

	decision := service.Authorize(context.Background(), Request{
		ActorID: actorID, WorkspaceID: currentWorkspaceID,
		NamespaceID: namespaceID, NodeID: nodeID, Permission: model.ResourcePermissionShare,
	})

	require.True(t, decision.Allowed)
	require.Equal(t, []uuid.UUID{grantID}, decision.SourceGrantIDs)
}

func TestAuthorizeCurrentWorkspaceAdminCannotBypassSourceWorkspace(t *testing.T) {
	organizationID, sourceWorkspaceID, currentWorkspaceID := uuid.New(), uuid.New(), uuid.New()
	actorID, namespaceID, nodeID := uuid.New(), uuid.New(), uuid.New()
	namespace := &model.Namespace{
		ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace,
		OrganizationID: organizationID, WorkspaceID: sourceWorkspaceID, PolicyVersion: 1,
	}
	target := &model.Node{ID: nodeID, NamespaceID: namespaceID, State: model.NodeStateActive}
	authority := workspaceAuthorityByIDStub{adminWorkspaceID: currentWorkspaceID, actorID: actorID}
	service := NewService(grantRepositoryStub{
		nodes:      map[uuid.UUID]*model.Node{nodeID: target},
		namespaces: map[uuid.UUID]*model.Namespace{namespaceID: namespace},
	}, principalResolverStub{principals: []repository.PrincipalRef{{Type: model.PrincipalUser, ID: actorID}}}, authority)

	decision := service.Authorize(context.Background(), Request{
		ActorID: actorID, WorkspaceID: currentWorkspaceID,
		NamespaceID: namespaceID, NodeID: nodeID, Permission: model.ResourcePermissionShare,
	})

	require.False(t, decision.Allowed)
	require.Equal(t, "default_deny", decision.Reason)
}

type workspaceAuthorityByIDStub struct {
	adminWorkspaceID uuid.UUID
	actorID          uuid.UUID
}

func (s workspaceAuthorityByIDStub) HasWorkspaceFileAdmin(_ context.Context, workspaceID, actorID uuid.UUID) (bool, error) {
	return workspaceID == s.adminWorkspaceID && actorID == s.actorID, nil
}

func (workspaceAuthorityByIDStub) HasOrganizationFileAdmin(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (r grantRepositoryStub) ListResourceGrants(_ context.Context, nodeIDs []uuid.UUID, principals []repository.PrincipalRef, permission model.ResourcePermission, at time.Time) ([]model.ResourceGrant, error) {
	var result []model.ResourceGrant
	for _, nodeID := range nodeIDs {
		for _, grant := range r.grants[nodeID] {
			for _, principal := range principals {
				if grant.PrincipalType == "" && principal.Type == model.PrincipalUser {
					grant.PrincipalType = model.PrincipalUser
					grant.PrincipalID = grant.GranteeID
				}
				if grant.PrincipalType == principal.Type && grant.PrincipalID == principal.ID && grant.Permission == permission && grant.RevokedAt == nil && (grant.ExpiresAt == nil || grant.ExpiresAt.After(at)) {
					result = append(result, grant)
				}
			}
		}
	}
	return result, nil
}

type authorityStub struct {
	workspaceAdmin    bool
	organizationAdmin bool
}

func (s authorityStub) HasWorkspaceFileAdmin(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.workspaceAdmin, nil
}

func (s authorityStub) HasOrganizationFileAdmin(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.organizationAdmin, nil
}

func TestAuthorizeNamespaceMatrixUsesResolvedPrincipals(t *testing.T) {
	organizationID, workspaceID := uuid.New(), uuid.New()
	userOwner, tenantOwner, member, roleID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	tests := []struct {
		name       string
		namespace  *model.Namespace
		actorID    uuid.UUID
		principals []repository.PrincipalRef
		grants     map[uuid.UUID][]model.ResourceGrant
		allowed    bool
	}{
		{
			name: "user namespace owner", actorID: userOwner, allowed: true,
			namespace: &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeUser, OrganizationID: organizationID, OwnerUserID: userOwner, PolicyVersion: 7},
		},
		{
			name: "tenant owner cannot read another user namespace", actorID: tenantOwner,
			namespace: &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeUser, OrganizationID: organizationID, OwnerUserID: userOwner, PolicyVersion: 7},
		},
		{
			name: "workspace grant covers active member", actorID: member, allowed: true,
			namespace: &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: organizationID, WorkspaceID: workspaceID, PolicyVersion: 8},
			principals: []repository.PrincipalRef{
				{Type: model.PrincipalUser, ID: member},
				{Type: model.PrincipalWorkspace, ID: workspaceID},
				{Type: model.PrincipalOrganization, ID: organizationID},
				{Type: model.PrincipalRole, ID: roleID},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := node(tt.namespace.ID, uuid.Nil, uuid.New())
			grants := tt.grants
			if tt.name == "workspace grant covers active member" {
				grants = map[uuid.UUID][]model.ResourceGrant{root.ID: {{
					NodeID: root.ID, PrincipalType: model.PrincipalWorkspace, PrincipalID: workspaceID,
					Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
				}}}
			}
			resolver := principalResolverStub{principals: tt.principals}
			if tt.namespace.NamespaceType == model.NamespaceTypeUser {
				resolver.principals = []repository.PrincipalRef{{Type: model.PrincipalUser, ID: tt.actorID}}
			}
			service := NewService(grantRepositoryStub{
				nodes: map[uuid.UUID]*model.Node{root.ID: root}, namespaces: map[uuid.UUID]*model.Namespace{tt.namespace.ID: tt.namespace}, grants: grants,
			}, resolver, authorityStub{organizationAdmin: true})

			decision := service.Authorize(context.Background(), Request{
				ActorID: tt.actorID, WorkspaceID: tt.namespace.WorkspaceID, NamespaceID: tt.namespace.ID,
				NodeID: root.ID, Permission: model.ResourcePermissionRead,
			})

			require.Equal(t, tt.allowed, decision.Allowed)
		})
	}
}

type principalResolverStub struct {
	principals []repository.PrincipalRef
	err        error
}

func (s principalResolverStub) Resolve(context.Context, ResolvePrincipalsInput) ([]repository.PrincipalRef, error) {
	return s.principals, s.err
}

func TestAuthorizeOrganizationGrantRequiresValidWorkspaceContext(t *testing.T) {
	organizationID, workspaceID, actorID := uuid.New(), uuid.New(), uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization, OrganizationID: organizationID, PolicyVersion: 4}
	root := node(namespace.ID, uuid.Nil, uuid.New())
	repo := grantRepositoryStub{
		nodes: map[uuid.UUID]*model.Node{root.ID: root}, namespaces: map[uuid.UUID]*model.Namespace{namespace.ID: namespace},
		grants: map[uuid.UUID][]model.ResourceGrant{root.ID: {{
			NodeID: root.ID, PrincipalType: model.PrincipalOrganization, PrincipalID: organizationID,
			Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
		}}},
	}
	valid := NewService(repo, principalResolverStub{principals: []repository.PrincipalRef{
		{Type: model.PrincipalUser, ID: actorID},
		{Type: model.PrincipalWorkspace, ID: workspaceID},
		{Type: model.PrincipalOrganization, ID: organizationID},
	}}, authorityStub{})
	require.True(t, valid.Authorize(context.Background(), Request{
		ActorID: actorID, WorkspaceID: workspaceID, NamespaceID: namespace.ID, NodeID: root.ID, Permission: model.ResourcePermissionRead,
	}).Allowed)

	withoutWorkspace := NewService(repo, principalResolverStub{
		principals: []repository.PrincipalRef{{Type: model.PrincipalUser, ID: actorID}},
	}, authorityStub{})
	require.False(t, withoutWorkspace.Authorize(context.Background(), Request{
		ActorID: actorID, NamespaceID: namespace.ID, NodeID: root.ID, Permission: model.ResourcePermissionRead,
	}).Allowed)
}

func TestAuthorizeDirectDenyOverridesInheritedOrganizationAllow(t *testing.T) {
	organizationID, workspaceID, actorID := uuid.New(), uuid.New(), uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization, OrganizationID: organizationID, PolicyVersion: 3}
	root := node(namespace.ID, uuid.Nil, uuid.New())
	child := node(namespace.ID, root.ID, uuid.New())
	repo := grantRepositoryStub{
		nodes:      map[uuid.UUID]*model.Node{root.ID: root, child.ID: child},
		namespaces: map[uuid.UUID]*model.Namespace{namespace.ID: namespace},
		grants: map[uuid.UUID][]model.ResourceGrant{
			root.ID: {{
				NodeID: root.ID, PrincipalType: model.PrincipalOrganization, PrincipalID: organizationID,
				InheritToChildren: true, Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
			}},
			child.ID: {{
				NodeID: child.ID, PrincipalType: model.PrincipalUser, PrincipalID: actorID,
				Permission: model.ResourcePermissionRead, Effect: model.GrantEffectDeny,
			}},
		},
	}
	service := NewService(repo, principalResolverStub{principals: []repository.PrincipalRef{
		{Type: model.PrincipalUser, ID: actorID},
		{Type: model.PrincipalOrganization, ID: organizationID},
	}}, authorityStub{})

	decision := service.Authorize(context.Background(), Request{
		ActorID: actorID, WorkspaceID: workspaceID, NamespaceID: namespace.ID, NodeID: child.ID, Permission: model.ResourcePermissionRead,
	})

	require.False(t, decision.Allowed)
	require.Equal(t, "explicit_deny", decision.Reason)
}

func TestAuthorizeDecisionIncludesAncestorDenyAndChildAllowSources(t *testing.T) {
	organizationID, workspaceID, actorID := uuid.New(), uuid.New(), uuid.New()
	namespace := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization, OrganizationID: organizationID, PolicyVersion: 5}
	root := node(namespace.ID, uuid.Nil, uuid.New())
	child := node(namespace.ID, root.ID, uuid.New())
	denyID, allowID := uuid.New(), uuid.New()
	service := NewService(grantRepositoryStub{
		nodes:      map[uuid.UUID]*model.Node{root.ID: root, child.ID: child},
		namespaces: map[uuid.UUID]*model.Namespace{namespace.ID: namespace},
		grants: map[uuid.UUID][]model.ResourceGrant{
			root.ID: {{
				ID: denyID, NodeID: root.ID, PrincipalType: model.PrincipalUser, PrincipalID: actorID,
				InheritToChildren: true, Permission: model.ResourcePermissionRead, Effect: model.GrantEffectDeny,
			}},
			child.ID: {{
				ID: allowID, NodeID: child.ID, PrincipalType: model.PrincipalUser, PrincipalID: actorID,
				Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
			}},
		},
	}, principalResolverStub{principals: []repository.PrincipalRef{{Type: model.PrincipalUser, ID: actorID}}}, authorityStub{})

	decision := service.Authorize(context.Background(), Request{
		ActorID: actorID, WorkspaceID: workspaceID, NamespaceID: namespace.ID,
		NodeID: child.ID, Permission: model.ResourcePermissionRead,
	})

	require.False(t, decision.Allowed)
	require.Equal(t, "explicit_deny", decision.Reason)
	require.ElementsMatch(t, []uuid.UUID{denyID, allowID}, decision.SourceGrantIDs)
}

func TestAuthorizeDecisionUsesRepositoryNamespacePolicyVersionInCacheKey(t *testing.T) {
	organizationID, ownerID := uuid.New(), uuid.New()
	request := Request{
		ActorID: ownerID, WorkspaceID: uuid.New(), NamespaceID: uuid.New(),
		NodeID: uuid.New(), Permission: model.ResourcePermissionWrite,
	}
	namespace := &model.Namespace{
		ID: request.NamespaceID, NamespaceType: model.NamespaceTypeUser,
		OrganizationID: organizationID, OwnerUserID: ownerID, PolicyVersion: 11,
	}
	target := node(namespace.ID, uuid.Nil, ownerID)
	target.ID = request.NodeID
	service := NewService(grantRepositoryStub{
		nodes:      map[uuid.UUID]*model.Node{target.ID: target},
		namespaces: map[uuid.UUID]*model.Namespace{namespace.ID: namespace},
	}, nil, nil)

	first := service.Authorize(context.Background(), request)
	namespace.PolicyVersion = 12
	second := service.Authorize(context.Background(), request)

	require.True(t, first.Allowed)
	require.True(t, second.Allowed)
	require.Equal(t, int64(11), first.PolicyVersion)
	require.Equal(t, int64(12), second.PolicyVersion)
	require.NotEqual(t, first.CacheKey, second.CacheKey)
	require.Equal(t, AuthorizationCacheKey{
		OrganizationID: organizationID,
		WorkspaceID:    request.WorkspaceID,
		ActorID:        ownerID,
		NodeID:         request.NodeID,
		Action:         model.ResourcePermissionWrite,
		PolicyVersion:  11,
	}, first.CacheKey)
}

func TestAuthorizeInheritsAllowFromAncestor(t *testing.T) {
	namespaceID, owner, actor := uuid.New(), uuid.New(), uuid.New()
	root := node(namespaceID, uuid.Nil, owner)
	child := node(namespaceID, root.ID, owner)
	service := NewService(grantRepositoryStub{
		nodes:      map[uuid.UUID]*model.Node{root.ID: root, child.ID: child},
		namespaces: map[uuid.UUID]*model.Namespace{namespaceID: userNamespace(namespaceID, owner)},
		grants: map[uuid.UUID][]model.ResourceGrant{
			root.ID: {{NodeID: root.ID, GranteeID: actor, InheritToChildren: true, Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow}},
		},
	}, nil, nil)

	decision := service.Authorize(context.Background(), Request{ActorID: actor, NamespaceID: namespaceID, NodeID: child.ID, Permission: model.ResourcePermissionRead})

	require.True(t, decision.Allowed)
	require.Equal(t, "grant_allow", decision.Reason)
}

func TestAuthorizeExplicitDenyWinsAcrossAncestors(t *testing.T) {
	namespaceID, owner, actor := uuid.New(), uuid.New(), uuid.New()
	root := node(namespaceID, uuid.Nil, owner)
	child := node(namespaceID, root.ID, owner)
	service := NewService(grantRepositoryStub{
		nodes:      map[uuid.UUID]*model.Node{root.ID: root, child.ID: child},
		namespaces: map[uuid.UUID]*model.Namespace{namespaceID: userNamespace(namespaceID, owner)},
		grants: map[uuid.UUID][]model.ResourceGrant{
			root.ID:  {{NodeID: root.ID, GranteeID: actor, InheritToChildren: true, Permission: model.ResourcePermissionWrite, Effect: model.GrantEffectAllow}},
			child.ID: {{NodeID: child.ID, GranteeID: actor, Permission: model.ResourcePermissionWrite, Effect: model.GrantEffectDeny}},
		},
	}, nil, nil)

	decision := service.Authorize(context.Background(), Request{ActorID: actor, NamespaceID: namespaceID, NodeID: child.ID, Permission: model.ResourcePermissionWrite})

	require.False(t, decision.Allowed)
	require.Equal(t, "explicit_deny", decision.Reason)
}

func TestAuthorizeHonorsGrantInheritanceFlag(t *testing.T) {
	namespaceID, owner, actor := uuid.New(), uuid.New(), uuid.New()
	root := node(namespaceID, uuid.Nil, owner)
	child := node(namespaceID, root.ID, owner)
	repo := grantRepositoryStub{
		nodes:      map[uuid.UUID]*model.Node{root.ID: root, child.ID: child},
		namespaces: map[uuid.UUID]*model.Namespace{namespaceID: userNamespace(namespaceID, owner)},
		grants: map[uuid.UUID][]model.ResourceGrant{
			root.ID:  {{NodeID: root.ID, GranteeID: actor, Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow}},
			child.ID: {{NodeID: child.ID, GranteeID: actor, Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow}},
		},
	}
	service := NewService(repo, nil, nil)

	require.True(t, service.Authorize(context.Background(), Request{
		ActorID: actor, NamespaceID: namespaceID, NodeID: child.ID, Permission: model.ResourcePermissionRead,
	}).Allowed)
	repo.grants[child.ID] = nil
	require.False(t, service.Authorize(context.Background(), Request{
		ActorID: actor, NamespaceID: namespaceID, NodeID: child.ID, Permission: model.ResourcePermissionRead,
	}).Allowed)
	require.True(t, service.Authorize(context.Background(), Request{
		ActorID: actor, NamespaceID: namespaceID, NodeID: root.ID, Permission: model.ResourcePermissionRead,
	}).Allowed)
}

func TestAuthorizeAdminGrantCoversEveryPermissionAndAdminDenyWins(t *testing.T) {
	namespaceID, owner, actor := uuid.New(), uuid.New(), uuid.New()
	root := node(namespaceID, uuid.Nil, owner)
	child := node(namespaceID, root.ID, owner)
	repo := grantRepositoryStub{
		nodes:      map[uuid.UUID]*model.Node{root.ID: root, child.ID: child},
		namespaces: map[uuid.UUID]*model.Namespace{namespaceID: userNamespace(namespaceID, owner)},
		grants: map[uuid.UUID][]model.ResourceGrant{
			root.ID: {{NodeID: root.ID, GranteeID: actor, InheritToChildren: true, Permission: model.ResourcePermissionAdmin, Effect: model.GrantEffectAllow}},
		},
	}
	service := NewService(repo, nil, nil)
	for _, permission := range []model.ResourcePermission{
		model.ResourcePermissionRead, model.ResourcePermissionWrite, model.ResourcePermissionDelete,
		model.ResourcePermissionShare, model.ResourcePermissionAdmin,
	} {
		require.True(t, service.Authorize(context.Background(), Request{ActorID: actor, NamespaceID: namespaceID, NodeID: child.ID, Permission: permission}).Allowed, permission)
	}
	repo.grants[child.ID] = []model.ResourceGrant{
		{NodeID: child.ID, GranteeID: actor, Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow},
		{NodeID: child.ID, GranteeID: actor, Permission: model.ResourcePermissionAdmin, Effect: model.GrantEffectDeny},
	}
	decision := NewService(repo, nil, nil).Authorize(context.Background(), Request{ActorID: actor, NamespaceID: namespaceID, NodeID: child.ID, Permission: model.ResourcePermissionRead})
	require.False(t, decision.Allowed)
	require.Equal(t, "admin_deny", decision.Reason)
}

func TestAuthorizeOwnerAdminDefaultDenyAndNamespaceBoundary(t *testing.T) {
	namespaceID, workspaceID, owner, stranger := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	target := node(namespaceID, uuid.Nil, owner)
	userNS := userNamespace(namespaceID, owner)
	userService := NewService(grantRepositoryStub{
		nodes:      map[uuid.UUID]*model.Node{target.ID: target},
		namespaces: map[uuid.UUID]*model.Namespace{namespaceID: userNS},
	}, nil, authorityStub{organizationAdmin: true})

	require.Equal(t, "owner", userService.Authorize(context.Background(), Request{ActorID: owner, NamespaceID: namespaceID, NodeID: target.ID, Permission: model.ResourcePermissionWrite}).Reason)
	require.Equal(t, "owner", userService.Authorize(context.Background(), Request{ActorID: owner, NamespaceID: namespaceID, NodeID: target.ID, Permission: model.ResourcePermissionShare}).Reason)
	require.Equal(t, "default_deny", userService.Authorize(context.Background(), Request{ActorID: stranger, NamespaceID: namespaceID, NodeID: target.ID, Permission: model.ResourcePermissionAdmin}).Reason)

	workspaceNS := &model.Namespace{
		ID: namespaceID, NamespaceType: model.NamespaceTypeWorkspace,
		OrganizationID: uuid.New(), WorkspaceID: workspaceID, PolicyVersion: 1,
	}
	workspaceService := NewService(grantRepositoryStub{
		nodes:      map[uuid.UUID]*model.Node{target.ID: target},
		namespaces: map[uuid.UUID]*model.Namespace{namespaceID: workspaceNS},
	}, nil, authorityStub{workspaceAdmin: true})
	require.Equal(t, "admin_bypass", workspaceService.Authorize(context.Background(), Request{
		ActorID: stranger, WorkspaceID: workspaceID, NamespaceID: namespaceID, NodeID: target.ID, Permission: model.ResourcePermissionAdmin,
	}).Reason)
	require.Equal(t, "admin_bypass", workspaceService.Authorize(context.Background(), Request{
		ActorID: stranger, WorkspaceID: workspaceID, NamespaceID: namespaceID, NodeID: target.ID, Permission: model.ResourcePermissionShare,
	}).Reason)

	withoutAdmin := NewService(grantRepositoryStub{
		nodes:      map[uuid.UUID]*model.Node{target.ID: target},
		namespaces: map[uuid.UUID]*model.Namespace{namespaceID: userNS},
	}, nil, nil)
	require.Equal(t, "default_deny", withoutAdmin.Authorize(context.Background(), Request{ActorID: stranger, NamespaceID: namespaceID, NodeID: target.ID, Permission: model.ResourcePermissionRead}).Reason)
	require.Equal(t, "default_deny", withoutAdmin.Authorize(context.Background(), Request{ActorID: stranger, NamespaceID: namespaceID, NodeID: target.ID, Permission: model.ResourcePermissionShare}).Reason)
	require.Equal(t, "resource_not_found", withoutAdmin.Authorize(context.Background(), Request{ActorID: owner, NamespaceID: uuid.New(), NodeID: target.ID, Permission: model.ResourcePermissionRead}).Reason)
}

func userNamespace(namespaceID, ownerID uuid.UUID) *model.Namespace {
	return &model.Namespace{
		ID: namespaceID, NamespaceType: model.NamespaceTypeUser,
		OrganizationID: uuid.New(), OwnerUserID: ownerID, PolicyVersion: 1,
	}
}

func node(namespaceID, parentID, owner uuid.UUID) *model.Node {
	parent := &parentID
	if parentID == uuid.Nil {
		parent = nil
	}
	return &model.Node{ID: uuid.New(), NamespaceID: namespaceID, ParentID: parent, Type: model.NodeTypeDirectory, State: model.NodeStateActive, OwnerID: owner}
}

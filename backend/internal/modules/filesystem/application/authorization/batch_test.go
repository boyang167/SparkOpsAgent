package authorization

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type instrumentedBatchRepository struct {
	grantRepositoryStub
	nodeBatchCalls  int
	grantBatchCalls int
}

func (r *instrumentedBatchRepository) GetNodesWithAncestors(_ context.Context, namespaceID uuid.UUID, _ []uuid.UUID) ([]model.Node, error) {
	r.nodeBatchCalls++
	nodes := make([]model.Node, 0, len(r.nodes))
	for _, candidate := range r.nodes {
		if candidate.NamespaceID == namespaceID {
			nodes = append(nodes, *candidate)
		}
	}
	return nodes, nil
}

func (r *instrumentedBatchRepository) ListResourceGrantsForActions(
	_ context.Context,
	nodeIDs []uuid.UUID,
	principals []repository.PrincipalRef,
	actions []model.ResourcePermission,
	_ time.Time,
) ([]model.ResourceGrant, error) {
	r.grantBatchCalls++
	nodes := make(map[uuid.UUID]struct{}, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		nodes[nodeID] = struct{}{}
	}
	principalSet := make(map[repository.PrincipalRef]struct{}, len(principals))
	for _, principal := range principals {
		principalSet[principal] = struct{}{}
	}
	actionSet := make(map[model.ResourcePermission]struct{}, len(actions))
	for _, action := range actions {
		actionSet[action] = struct{}{}
	}
	var result []model.ResourceGrant
	for nodeID, grants := range r.grants {
		if _, ok := nodes[nodeID]; !ok {
			continue
		}
		for _, grant := range grants {
			if _, ok := principalSet[repository.PrincipalRef{Type: grant.PrincipalType, ID: grant.PrincipalID}]; !ok {
				continue
			}
			if _, ok := actionSet[grant.Permission]; ok {
				result = append(result, grant)
			}
		}
	}
	return result, nil
}

func TestAuthorizeManyLoadsAncestorsAndGrantsOnceForLargeBatch(t *testing.T) {
	organizationID, workspaceID, actorID := uuid.New(), uuid.New(), uuid.New()
	namespace := &model.Namespace{
		ID: uuid.New(), NamespaceType: model.NamespaceTypeOrganization,
		OrganizationID: organizationID, PolicyVersion: 11,
	}
	root := node(namespace.ID, uuid.Nil, actorID)
	denyID := uuid.New()
	repo := &instrumentedBatchRepository{
		grantRepositoryStub: grantRepositoryStub{
			nodes:      map[uuid.UUID]*model.Node{root.ID: root},
			namespaces: map[uuid.UUID]*model.Namespace{namespace.ID: namespace},
			grants: map[uuid.UUID][]model.ResourceGrant{
				root.ID: {
					{
						ID: denyID, NodeID: root.ID, PrincipalType: model.PrincipalUser, PrincipalID: actorID,
						Permission: model.ResourcePermissionRead, Effect: model.GrantEffectDeny, InheritToChildren: true,
					},
				},
			},
		},
	}
	nodeIDs := make([]uuid.UUID, 300)
	allowIDs := make(map[uuid.UUID]uuid.UUID, len(nodeIDs))
	for index := range nodeIDs {
		child := node(namespace.ID, root.ID, actorID)
		nodeIDs[index] = child.ID
		repo.nodes[child.ID] = child
		allowID := uuid.New()
		allowIDs[child.ID] = allowID
		repo.grants[child.ID] = []model.ResourceGrant{{
			ID: allowID, NodeID: child.ID, PrincipalType: model.PrincipalUser, PrincipalID: actorID,
			Permission: model.ResourcePermissionRead, Effect: model.GrantEffectAllow,
		}}
	}
	service := NewService(repo, principalResolverStub{principals: []repository.PrincipalRef{{
		Type: model.PrincipalUser, ID: actorID,
	}}}, authorityStub{})

	decisions := service.AuthorizeMany(context.Background(), BatchRequest{
		ActorID: actorID, WorkspaceID: workspaceID, NamespaceID: namespace.ID,
		NodeIDs: nodeIDs, Permissions: []model.ResourcePermission{model.ResourcePermissionRead},
	})

	require.Equal(t, 1, repo.nodeBatchCalls)
	require.Equal(t, 1, repo.grantBatchCalls)
	for _, nodeID := range nodeIDs {
		decision := decisions[nodeID][model.ResourcePermissionRead]
		require.False(t, decision.Allowed)
		require.Equal(t, "explicit_deny", decision.Reason)
		require.ElementsMatch(t, []uuid.UUID{denyID, allowIDs[nodeID]}, decision.SourceGrantIDs)
	}
}

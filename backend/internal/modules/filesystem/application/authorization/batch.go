package authorization

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	domainservice "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
)

type BatchRequest struct {
	ActorID     uuid.UUID
	WorkspaceID uuid.UUID
	NamespaceID uuid.UUID
	NodeIDs     []uuid.UUID
	Permissions []model.ResourcePermission
}

type BatchDecisions map[uuid.UUID]map[model.ResourcePermission]Decision

type BatchRepository interface {
	GetNodesWithAncestors(context.Context, uuid.UUID, []uuid.UUID) ([]model.Node, error)
	ListResourceGrantsForActions(context.Context, []uuid.UUID, []repository.PrincipalRef, []model.ResourcePermission, time.Time) ([]model.ResourceGrant, error)
}

func (s *Service) AuthorizeMany(ctx context.Context, request BatchRequest) BatchDecisions {
	out := emptyBatchDecisions(request)
	batchRepo, ok := s.repo.(BatchRepository)
	if !ok {
		return batchDenied(out, "authorization_unavailable")
	}
	namespace, err := s.repo.GetNamespace(ctx, request.NamespaceID)
	if err != nil {
		return batchDenied(out, "resource_not_found")
	}
	if namespace.NamespaceType == model.NamespaceTypeWorkspace && namespace.WorkspaceID != request.WorkspaceID {
		return batchDenied(out, "organization_boundary")
	}
	principals := []repository.PrincipalRef{{Type: model.PrincipalUser, ID: request.ActorID}}
	if s.principals != nil {
		principals, err = s.principals.Resolve(ctx, ResolvePrincipalsInput{
			ActorID: request.ActorID, OrganizationID: namespace.OrganizationID, WorkspaceID: request.WorkspaceID,
		})
		if errors.Is(err, ErrPrincipalScopeMismatch) {
			return batchDenied(out, "organization_boundary")
		}
		if err != nil {
			return batchDenied(out, "authorization_unavailable")
		}
	}
	nodes, err := batchRepo.GetNodesWithAncestors(ctx, request.NamespaceID, request.NodeIDs)
	if err != nil {
		return batchDenied(out, "authorization_unavailable")
	}
	nodeByID := make(map[uuid.UUID]model.Node, len(nodes))
	ancestorIDs := make([]uuid.UUID, 0, len(nodes))
	for _, node := range nodes {
		nodeByID[node.ID] = node
		ancestorIDs = append(ancestorIDs, node.ID)
	}
	grantActions := append([]model.ResourcePermission{}, request.Permissions...)
	if !containsPermission(grantActions, model.ResourcePermissionAdmin) {
		grantActions = append(grantActions, model.ResourcePermissionAdmin)
	}
	grants, err := batchRepo.ListResourceGrantsForActions(ctx, ancestorIDs, principals, grantActions, s.now())
	if err != nil {
		return batchDenied(out, "authorization_unavailable")
	}
	grantsByNode := make(map[uuid.UUID][]model.ResourceGrant)
	for _, grant := range grants {
		grantsByNode[grant.NodeID] = append(grantsByNode[grant.NodeID], grant)
	}
	admin := false
	if s.authority != nil {
		admin, err = s.hasNamespaceAuthority(ctx, namespace, Request{ActorID: request.ActorID})
		if err != nil {
			return batchDenied(out, "authorization_unavailable")
		}
	}
	for _, nodeID := range request.NodeIDs {
		target, exists := nodeByID[nodeID]
		if !exists || target.State != model.NodeStateActive {
			setBatchReason(out[nodeID], "resource_not_found")
			continue
		}
		for _, permission := range request.Permissions {
			out[nodeID][permission] = evaluateBatchDecision(
				request, namespace, target, permission, nodeByID, grantsByNode, admin,
			)
		}
	}
	return out
}

func evaluateBatchDecision(
	request BatchRequest,
	namespace *model.Namespace,
	target model.Node,
	permission model.ResourcePermission,
	nodes map[uuid.UUID]model.Node,
	grants map[uuid.UUID][]model.ResourceGrant,
	admin bool,
) Decision {
	decision := Decision{
		PolicyVersion: namespace.PolicyVersion,
		CacheKey: authorizationCacheKey(Request{
			ActorID: request.ActorID, WorkspaceID: request.WorkspaceID,
			NodeID: target.ID, Permission: permission,
		}, namespace.OrganizationID, namespace.PolicyVersion),
	}
	relations := make([]domainservice.Relation, 0)
	sourceIDs := make(map[uuid.UUID]struct{})
	current := target
	for {
		for _, grant := range grants[current.ID] {
			if grant.Permission != permission && grant.Permission != model.ResourcePermissionAdmin {
				continue
			}
			if current.ID != target.ID && !grant.InheritToChildren {
				continue
			}
			relations = append(relations, domainservice.Relation{
				ResourceID: current.ID, Effect: grant.Effect, Inherited: current.ID != target.ID,
				Permission: grant.Permission, Source: domainservice.SourceGrant,
				PolicyID: grant.ID.String(), PolicyVersion: strconv.FormatInt(grant.PolicyVersion, 10),
			})
			sourceIDs[grant.ID] = struct{}{}
		}
		if current.ParentID == nil {
			break
		}
		parent, exists := nodes[*current.ParentID]
		if !exists || parent.State != model.NodeStateActive {
			decision.Reason = "resource_not_found"
			return decision
		}
		current = parent
	}
	if namespace.NamespaceType == model.NamespaceTypeUser && namespace.OwnerUserID == request.ActorID {
		relations = append(relations, domainservice.Relation{
			ResourceID: target.ID, Effect: model.GrantEffectAllow, Permission: permission,
			Source: domainservice.SourceOwner, PolicyID: "owner",
		})
	}
	if admin {
		relations = append(relations, domainservice.Relation{
			ResourceID: target.ID, Effect: model.GrantEffectAllow, Permission: permission,
			Source: domainservice.SourceAdminBypass, PolicyID: "admin_bypass",
		})
	}
	decided := domainservice.Decide(domainservice.Subject{
		ActorID: request.ActorID, OrganizationID: namespace.OrganizationID,
		WorkspaceID: request.WorkspaceID, Authenticated: true, Entitled: true, RBACAllowed: true,
	}, permission, domainservice.Resource{
		ID: target.ID, OrganizationID: namespace.OrganizationID, WorkspaceID: request.WorkspaceID,
	}, relations...)
	decision.Allowed = decided.Result == domainservice.DecisionAllow
	decision.Reason = decided.ReasonCode
	decision.SourceGrantIDs = sortedGrantIDs(sourceIDs)
	return decision
}

func emptyBatchDecisions(request BatchRequest) BatchDecisions {
	out := make(BatchDecisions, len(request.NodeIDs))
	for _, nodeID := range request.NodeIDs {
		out[nodeID] = make(map[model.ResourcePermission]Decision, len(request.Permissions))
		for _, permission := range request.Permissions {
			out[nodeID][permission] = Decision{}
		}
	}
	return out
}

func batchDenied(out BatchDecisions, reason string) BatchDecisions {
	for _, decisions := range out {
		setBatchReason(decisions, reason)
	}
	return out
}

func setBatchReason(decisions map[model.ResourcePermission]Decision, reason string) {
	for permission, decision := range decisions {
		decision.Reason = reason
		decisions[permission] = decision
	}
}

func containsPermission(actions []model.ResourcePermission, action model.ResourcePermission) bool {
	for _, candidate := range actions {
		if candidate == action {
			return true
		}
	}
	return false
}

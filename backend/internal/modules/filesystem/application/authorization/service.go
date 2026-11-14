package authorization

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	domainservice "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
)

type Repository interface {
	GetNamespace(context.Context, uuid.UUID) (*model.Namespace, error)
	GetNode(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	GetNodeIncludingTrashed(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	ListResourceGrants(context.Context, []uuid.UUID, []repository.PrincipalRef, model.ResourcePermission, time.Time) ([]model.ResourceGrant, error)
}

type AuthorityChecker interface {
	HasWorkspaceFileAdmin(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	HasOrganizationFileAdmin(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

type Request struct {
	ActorID     uuid.UUID
	WorkspaceID uuid.UUID
	NamespaceID uuid.UUID
	NodeID      uuid.UUID
	Permission  model.ResourcePermission
}

type Decision struct {
	Allowed bool
	Reason  string
}

type AuthorizationCacheKey struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	ActorID        uuid.UUID
	NodeID         uuid.UUID
	Action         model.ResourcePermission
	PolicyVersion  int64
}

type Service struct {
	repo       Repository
	principals PrincipalResolver
	authority  AuthorityChecker
	now        func() time.Time
}

func NewService(repo Repository, principals PrincipalResolver, authority AuthorityChecker) *Service {
	return &Service{repo: repo, principals: principals, authority: authority, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Authorize(ctx context.Context, request Request) Decision {
	namespace, err := s.repo.GetNamespace(ctx, request.NamespaceID)
	if err != nil {
		return Decision{Reason: "resource_not_found"}
	}
	if namespace.NamespaceType == model.NamespaceTypeWorkspace && namespace.WorkspaceID != request.WorkspaceID {
		return Decision{Reason: "organization_boundary"}
	}
	principals := []repository.PrincipalRef{{Type: model.PrincipalUser, ID: request.ActorID}}
	if s.principals != nil {
		principals, err = s.principals.Resolve(ctx, ResolvePrincipalsInput{
			ActorID: request.ActorID, OrganizationID: namespace.OrganizationID, WorkspaceID: request.WorkspaceID,
		})
		if errors.Is(err, ErrPrincipalScopeMismatch) {
			return Decision{Reason: "organization_boundary"}
		}
		if err != nil {
			return Decision{Reason: "authorization_unavailable"}
		}
	}
	getNode := s.repo.GetNode
	if request.Permission == model.ResourcePermissionDelete {
		getNode = s.repo.GetNodeIncludingTrashed
	}
	node, err := getNode(ctx, request.NamespaceID, request.NodeID)
	if err != nil {
		return Decision{Reason: "resource_not_found"}
	}
	relations := make([]domainservice.Relation, 0)
	for current := node; current != nil; {
		permissions := []model.ResourcePermission{request.Permission}
		if request.Permission != model.ResourcePermissionAdmin {
			permissions = append(permissions, model.ResourcePermissionAdmin)
		}
		for _, permission := range permissions {
			grants, grantErr := s.repo.ListResourceGrants(ctx, []uuid.UUID{current.ID}, principals, permission, s.now())
			if grantErr != nil {
				return Decision{Reason: "authorization_unavailable"}
			}
			for _, grant := range grants {
				if current.ID != node.ID && !grant.InheritToChildren {
					continue
				}
				relations = append(relations, domainservice.Relation{
					ResourceID: current.ID, Effect: grant.Effect, Inherited: current.ID != node.ID,
					Permission: grant.Permission, Source: domainservice.SourceGrant, PolicyID: grant.ID.String(),
					PolicyVersion: strconv.FormatInt(grant.PolicyVersion, 10),
				})
			}
		}
		if current.ParentID == nil {
			break
		}
		current, err = getNode(ctx, request.NamespaceID, *current.ParentID)
		if errors.Is(err, domain.ErrNodeNotFound) {
			return Decision{Reason: "resource_not_found"}
		}
		if err != nil {
			return Decision{Reason: "authorization_unavailable"}
		}
	}
	if namespace.NamespaceType == model.NamespaceTypeUser && namespace.OwnerUserID == request.ActorID {
		relations = append(relations, domainservice.Relation{
			ResourceID: node.ID, Effect: model.GrantEffectAllow, Permission: request.Permission,
			Source: domainservice.SourceOwner, PolicyID: "owner",
		})
	}
	if s.authority != nil {
		admin, adminErr := s.hasNamespaceAuthority(ctx, namespace, request)
		if adminErr != nil {
			return Decision{Reason: "authorization_unavailable"}
		}
		if admin {
			relations = append(relations, domainservice.Relation{
				ResourceID: node.ID, Effect: model.GrantEffectAllow, Permission: request.Permission,
				Source: domainservice.SourceAdminBypass, PolicyID: "admin_bypass",
			})
		}
	}
	decided := domainservice.Decide(domainservice.Subject{
		ActorID: request.ActorID, OrganizationID: namespace.OrganizationID, WorkspaceID: request.WorkspaceID,
		Authenticated: true, Entitled: true, RBACAllowed: true,
	}, request.Permission, domainservice.Resource{
		ID: node.ID, OrganizationID: namespace.OrganizationID, WorkspaceID: request.WorkspaceID,
	}, relations...)
	return Decision{Allowed: decided.Result == domainservice.DecisionAllow, Reason: decided.ReasonCode}
}

func (s *Service) hasNamespaceAuthority(ctx context.Context, namespace *model.Namespace, request Request) (bool, error) {
	switch namespace.NamespaceType {
	case model.NamespaceTypeUser:
		return namespace.OwnerUserID == request.ActorID, nil
	case model.NamespaceTypeWorkspace:
		return s.authority.HasWorkspaceFileAdmin(ctx, namespace.WorkspaceID, request.ActorID)
	case model.NamespaceTypeOrganization:
		return s.authority.HasOrganizationFileAdmin(ctx, namespace.OrganizationID, request.ActorID)
	default:
		return false, nil
	}
}

func authorizationCacheKey(request Request, organizationID uuid.UUID, policyVersion int64) AuthorizationCacheKey {
	return AuthorizationCacheKey{
		OrganizationID: organizationID,
		WorkspaceID:    request.WorkspaceID,
		ActorID:        request.ActorID,
		NodeID:         request.NodeID,
		Action:         request.Permission,
		PolicyVersion:  policyVersion,
	}
}

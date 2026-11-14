package authorization

import (
	"context"
	"errors"
	"sort"
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

type ReshareProofRepository interface {
	ListValidReshareProofGrants(context.Context, repository.ReshareProofQuery) ([]model.ResourceGrant, error)
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

type ReshareProofRequest struct {
	ActorID        uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	NodeID         uuid.UUID
	SourceGrantIDs []uuid.UUID
}

type Decision struct {
	Allowed        bool
	Reason         string
	PolicyVersion  int64
	CacheKey       AuthorizationCacheKey
	SourceGrantIDs []uuid.UUID
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
	decision := Decision{
		PolicyVersion: namespace.PolicyVersion,
		CacheKey:      authorizationCacheKey(request, namespace.OrganizationID, namespace.PolicyVersion),
	}
	denied := func(reason string) Decision {
		decision.Allowed = false
		decision.Reason = reason
		return decision
	}
	principals := []repository.PrincipalRef{{Type: model.PrincipalUser, ID: request.ActorID}}
	if s.principals != nil {
		principals, err = s.principals.Resolve(ctx, ResolvePrincipalsInput{
			ActorID: request.ActorID, OrganizationID: namespace.OrganizationID, WorkspaceID: request.WorkspaceID,
		})
		if errors.Is(err, ErrPrincipalScopeMismatch) {
			return denied("organization_boundary")
		}
		if err != nil {
			return denied("authorization_unavailable")
		}
	}
	getNode := s.repo.GetNode
	if request.Permission == model.ResourcePermissionDelete {
		getNode = s.repo.GetNodeIncludingTrashed
	}
	node, err := getNode(ctx, request.NamespaceID, request.NodeID)
	if err != nil {
		return denied("resource_not_found")
	}
	relations := make([]domainservice.Relation, 0)
	sourceGrantIDs := make(map[uuid.UUID]struct{})
	for current := node; current != nil; {
		permissions := []model.ResourcePermission{request.Permission}
		if request.Permission != model.ResourcePermissionAdmin {
			permissions = append(permissions, model.ResourcePermissionAdmin)
		}
		for _, permission := range permissions {
			grants, grantErr := s.repo.ListResourceGrants(ctx, []uuid.UUID{current.ID}, principals, permission, s.now())
			if grantErr != nil {
				return denied("authorization_unavailable")
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
				sourceGrantIDs[grant.ID] = struct{}{}
			}
		}
		if current.ParentID == nil {
			break
		}
		current, err = getNode(ctx, request.NamespaceID, *current.ParentID)
		if errors.Is(err, domain.ErrNodeNotFound) {
			return denied("resource_not_found")
		}
		if err != nil {
			return denied("authorization_unavailable")
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
			return denied("authorization_unavailable")
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
	decision.Allowed = decided.Result == domainservice.DecisionAllow
	decision.Reason = decided.ReasonCode
	decision.SourceGrantIDs = sortedGrantIDs(sourceGrantIDs)
	return decision
}

func (s *Service) HasIndependentShareAuthority(ctx context.Context, request Request) (bool, error) {
	namespace, err := s.repo.GetNamespace(ctx, request.NamespaceID)
	if err != nil {
		return false, err
	}
	if _, err := s.repo.GetNode(ctx, request.NamespaceID, request.NodeID); err != nil {
		return false, err
	}
	if namespace.NamespaceType == model.NamespaceTypeUser && namespace.OwnerUserID == request.ActorID {
		return true, nil
	}
	if s.authority == nil {
		return false, nil
	}
	return s.hasNamespaceAuthority(ctx, namespace, request)
}

func (s *Service) VerifyReshareProof(ctx context.Context, request ReshareProofRequest) error {
	if len(request.SourceGrantIDs) == 0 {
		return domain.ErrResourceDenied
	}
	namespace, err := s.repo.GetNamespace(ctx, request.NamespaceID)
	if err != nil {
		return domain.ErrResourceDenied
	}
	if _, err := s.repo.GetNode(ctx, request.NamespaceID, request.NodeID); err != nil {
		return domain.ErrResourceDenied
	}
	proofRepo, ok := s.repo.(ReshareProofRepository)
	if !ok || s.principals == nil {
		return domain.ErrResourceDenied
	}
	principals, err := s.principals.Resolve(ctx, ResolvePrincipalsInput{
		ActorID: request.ActorID, OrganizationID: namespace.OrganizationID, WorkspaceID: request.WorkspaceID,
	})
	if err != nil {
		return domain.ErrResourceDenied
	}
	grants, err := proofRepo.ListValidReshareProofGrants(ctx, repository.ReshareProofQuery{
		OrganizationID: namespace.OrganizationID, NamespaceID: namespace.ID, NodeID: request.NodeID,
		GrantIDs: request.SourceGrantIDs, Principals: principals, At: s.now(),
	})
	if err != nil {
		return err
	}
	if len(grants) != len(request.SourceGrantIDs) {
		return domain.ErrResourceDenied
	}
	return nil
}

func sortedGrantIDs(ids map[uuid.UUID]struct{}) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		if id != uuid.Nil {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
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

package authorization

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	domainservice "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
)

type Repository interface {
	GetNode(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	GetNodeIncludingTrashed(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	ListResourceGrants(context.Context, []uuid.UUID, []repository.PrincipalRef, model.ResourcePermission, time.Time) ([]model.ResourceGrant, error)
}

type AdminBypass func(context.Context, uuid.UUID, uuid.UUID) (bool, error)

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

type Service struct {
	repo        Repository
	adminBypass AdminBypass
	now         func() time.Time
}

func NewService(repo Repository, adminBypass AdminBypass) *Service {
	return &Service{repo: repo, adminBypass: adminBypass, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Authorize(ctx context.Context, request Request) Decision {
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
			grants, grantErr := s.repo.ListResourceGrants(ctx, []uuid.UUID{current.ID}, []repository.PrincipalRef{{Type: model.PrincipalUser, ID: request.ActorID}}, permission, s.now())
			if grantErr != nil {
				return Decision{Reason: "authorization_unavailable"}
			}
			for _, grant := range grants {
				relations = append(relations, domainservice.Relation{
					ResourceID: current.ID, Effect: grant.Effect, Inherited: current.ID != node.ID,
					Permission: grant.Permission, Source: domainservice.SourceGrant, PolicyID: grant.ID.String(),
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
	if node.OwnerID == request.ActorID {
		relations = append(relations, domainservice.Relation{
			ResourceID: node.ID, Effect: model.GrantEffectAllow, Permission: request.Permission,
			Source: domainservice.SourceOwner, PolicyID: "owner",
		})
	}
	if s.adminBypass != nil {
		admin, adminErr := s.adminBypass(ctx, request.WorkspaceID, request.ActorID)
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
		ActorID: request.ActorID, WorkspaceID: request.WorkspaceID,
		Authenticated: true, Entitled: true, RBACAllowed: true,
	}, request.Permission, domainservice.Resource{ID: node.ID, WorkspaceID: request.WorkspaceID}, relations...)
	return Decision{Allowed: decided.Result == domainservice.DecisionAllow, Reason: decided.ReasonCode}
}

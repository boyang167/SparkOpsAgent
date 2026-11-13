package authorization

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type Repository interface {
	GetNode(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	ListResourceGrants(context.Context, uuid.UUID, uuid.UUID, model.ResourcePermission, time.Time) ([]model.ResourceGrant, error)
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
	node, err := s.repo.GetNode(ctx, request.NamespaceID, request.NodeID)
	if err != nil {
		return Decision{Reason: "resource_not_found"}
	}
	owner := node.OwnerID == request.ActorID
	allowed := false
	for current := node; current != nil; {
		grants, grantErr := s.repo.ListResourceGrants(ctx, current.ID, request.ActorID, request.Permission, s.now())
		if grantErr != nil {
			return Decision{Reason: "authorization_unavailable"}
		}
		for _, grant := range grants {
			if grant.Effect == model.GrantEffectDeny {
				return Decision{Reason: "explicit_deny"}
			}
			if grant.Effect == model.GrantEffectAllow {
				allowed = true
			}
		}
		if current.ParentID == nil {
			break
		}
		current, err = s.repo.GetNode(ctx, request.NamespaceID, *current.ParentID)
		if errors.Is(err, domain.ErrNodeNotFound) {
			return Decision{Reason: "resource_not_found"}
		}
		if err != nil {
			return Decision{Reason: "authorization_unavailable"}
		}
	}
	if owner {
		return Decision{Allowed: true, Reason: "owner"}
	}
	if allowed {
		return Decision{Allowed: true, Reason: "grant_allow"}
	}
	if s.adminBypass != nil {
		admin, adminErr := s.adminBypass(ctx, request.WorkspaceID, request.ActorID)
		if adminErr != nil {
			return Decision{Reason: "authorization_unavailable"}
		}
		if admin {
			return Decision{Allowed: true, Reason: "admin_bypass"}
		}
	}
	return Decision{Reason: "default_deny"}
}

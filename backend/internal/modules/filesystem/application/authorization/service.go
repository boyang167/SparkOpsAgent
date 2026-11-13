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
	GetNodeIncludingTrashed(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
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
	getNode := s.repo.GetNode
	if request.Permission == model.ResourcePermissionDelete {
		getNode = s.repo.GetNodeIncludingTrashed
	}
	node, err := getNode(ctx, request.NamespaceID, request.NodeID)
	if err != nil {
		return Decision{Reason: "resource_not_found"}
	}
	owner := node.OwnerID == request.ActorID
	allowed := false
	exactDeny := false
	adminDeny := false
	for current := node; current != nil; {
		permissions := []model.ResourcePermission{request.Permission}
		if request.Permission != model.ResourcePermissionAdmin {
			permissions = append(permissions, model.ResourcePermissionAdmin)
		}
		for _, permission := range permissions {
			grants, grantErr := s.repo.ListResourceGrants(ctx, current.ID, request.ActorID, permission, s.now())
			if grantErr != nil {
				return Decision{Reason: "authorization_unavailable"}
			}
			for _, grant := range grants {
				if grant.Effect == model.GrantEffectDeny {
					if grant.Permission == model.ResourcePermissionAdmin {
						adminDeny = true
					} else {
						exactDeny = true
					}
				}
				if grant.Effect == model.GrantEffectAllow {
					allowed = true
				}
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
	if adminDeny {
		return Decision{Reason: "admin_deny"}
	}
	if exactDeny {
		return Decision{Reason: "explicit_deny"}
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

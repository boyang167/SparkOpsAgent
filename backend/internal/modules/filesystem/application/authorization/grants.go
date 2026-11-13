package authorization

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type GrantService struct {
	repo       repository.FilesystemRepository
	authorizer *Service
	tx         postgres.Transactor
	bus        messaging.Bus
	audit      audit.Logger
}

func NewGrantService(repo repository.FilesystemRepository, authorizer *Service, tx postgres.Transactor, bus messaging.Bus, logger audit.Logger) *GrantService {
	return &GrantService{repo: repo, authorizer: authorizer, tx: tx, bus: bus, audit: logger}
}

type GrantRequest struct {
	ActorID, OrganizationID, WorkspaceID, NamespaceID, NodeID uuid.UUID
	Grant                                                     model.ResourceGrant
	TraceID                                                   string
}

func (s *GrantService) Create(ctx context.Context, request GrantRequest) (model.ResourceGrant, error) {
	if decision := s.authorizer.Authorize(ctx, Request{ActorID: request.ActorID, WorkspaceID: request.WorkspaceID, NamespaceID: request.NamespaceID, NodeID: request.NodeID, Permission: model.ResourcePermissionAdmin}); !decision.Allowed {
		s.log(ctx, request, uuid.Nil, "create", "DENY", decision.Reason)
		return model.ResourceGrant{}, domain.ErrResourceDenied
	}
	grant := request.Grant
	if grant.ID == uuid.Nil {
		grant.ID = uuid.New()
	}
	grant.NodeID = request.NodeID
	err := s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repo.CreateResourceGrant(txCtx, grant); err != nil {
			return err
		}
		if s.bus != nil {
			if err := s.bus.Publish(txCtx, event.PermissionChanged{BaseEvent: events.NewBaseEvent(event.PermissionChangedName), NodeID: request.NodeID, GrantID: grant.ID, Action: "CREATED"}); err != nil {
				return err
			}
		}
		return s.log(txCtx, request, grant.ID, "create", "ALLOW", "")
	})
	return grant, err
}

func (s *GrantService) Revoke(ctx context.Context, request GrantRequest, grantID uuid.UUID) error {
	if decision := s.authorizer.Authorize(ctx, Request{ActorID: request.ActorID, WorkspaceID: request.WorkspaceID, NamespaceID: request.NamespaceID, NodeID: request.NodeID, Permission: model.ResourcePermissionAdmin}); !decision.Allowed {
		s.log(ctx, request, grantID, "revoke", "DENY", decision.Reason)
		return domain.ErrResourceDenied
	}
	return s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repo.RevokeResourceGrant(txCtx, request.NodeID, grantID); err != nil {
			return err
		}
		if s.bus != nil {
			if err := s.bus.Publish(txCtx, event.PermissionChanged{BaseEvent: events.NewBaseEvent(event.PermissionChangedName), NodeID: request.NodeID, GrantID: grantID, Action: "REVOKED"}); err != nil {
				return err
			}
		}
		return s.log(txCtx, request, grantID, "revoke", "ALLOW", "")
	})
}

func (s *GrantService) List(ctx context.Context, request GrantRequest) ([]model.ResourceGrant, error) {
	if decision := s.authorizer.Authorize(ctx, Request{ActorID: request.ActorID, WorkspaceID: request.WorkspaceID, NamespaceID: request.NamespaceID, NodeID: request.NodeID, Permission: model.ResourcePermissionAdmin}); !decision.Allowed {
		return nil, domain.ErrResourceDenied
	}
	return s.repo.ListNodeResourceGrants(ctx, request.NodeID)
}

func (s *GrantService) log(ctx context.Context, request GrantRequest, grantID uuid.UUID, operation, decision, reason string) error {
	if s.audit == nil {
		return nil
	}
	detail := map[string]any{"workspace_id": request.WorkspaceID.String(), "decision": decision, "trace_id": request.TraceID, "grant_id": grantID.String()}
	if reason != "" {
		detail["reason"] = reason
	}
	return s.audit.Log(ctx, audit.Entry{
		ActorID: kernel.MemberID{UUID: request.ActorID}, OrgID: &kernel.OrgID{UUID: request.OrganizationID},
		Action: "filesystem.grant." + operation, ResourceType: "filesystem", ResourceID: request.NodeID.String(), Detail: detail,
	})
}

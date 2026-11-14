package authorization

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	filesystemport "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
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

type GrantRequest = filesystemport.GrantRequest

func (s *GrantService) Create(ctx context.Context, request GrantRequest) (model.ResourceGrant, error) {
	request, err := s.authoritativeRequest(ctx, request)
	if err != nil {
		s.log(ctx, request, uuid.Nil, "create", "DENY", "tenant_mismatch")
		return model.ResourceGrant{}, domain.ErrResourceDenied
	}
	if request.CommandID == uuid.Nil {
		s.log(ctx, request, uuid.Nil, "create", "DENY", "idempotency_required")
		return model.ResourceGrant{}, domain.ErrIdempotencyConflict
	}
	if decision := s.authorizer.Authorize(ctx, Request{ActorID: request.ActorID, WorkspaceID: request.WorkspaceID, NamespaceID: request.NamespaceID, NodeID: request.NodeID, Permission: model.ResourcePermissionAdmin}); !decision.Allowed {
		s.log(ctx, request, uuid.Nil, "create", "DENY", decision.Reason)
		return model.ResourceGrant{}, domain.ErrResourceDenied
	}
	grant := request.Grant
	if grant.PrincipalID != uuid.Nil && grant.GranteeID != uuid.Nil && grant.PrincipalID != grant.GranteeID {
		return model.ResourceGrant{}, domain.ErrResourceDenied
	}
	if grant.PrincipalType == "" {
		grant.PrincipalType = model.PrincipalUser
	}
	if grant.PrincipalID == uuid.Nil {
		grant.PrincipalID = grant.GranteeID
	}
	if grant.GranteeID == uuid.Nil {
		grant.GranteeID = grant.PrincipalID
	}
	if grant.PrincipalID == uuid.Nil {
		return model.ResourceGrant{}, domain.ErrResourceDenied
	}
	if request.GranteeOrganizationID != uuid.Nil && request.GranteeOrganizationID != request.OrganizationID {
		return model.ResourceGrant{}, domain.ErrCrossOrganizationGrant
	}
	if grant.ID == uuid.Nil {
		if request.CommandID != uuid.Nil {
			grant.ID = uuid.NewSHA1(uuid.NameSpaceOID, []byte("filesystem-grant:"+request.CommandID.String()))
		} else {
			grant.ID = uuid.New()
		}
	}
	grant.NodeID = request.NodeID
	grant.OrganizationID = request.OrganizationID
	if !grant.InheritToChildren {
		grant.InheritToChildren = true
	}
	if grant.PolicyVersion == 0 {
		grant.PolicyVersion = 1
	}
	request.Grant = grant
	requestHash := grantRequestHash("grant_create", request, grant.ID)
	err = s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		replayed, claimErr := s.claim(txCtx, request, "grant_create", requestHash, grant.ID)
		if claimErr != nil || replayed {
			return claimErr
		}
		if err := s.repo.CreateResourceGrant(txCtx, grant); err != nil {
			return err
		}
		if s.bus != nil {
			if err := s.bus.Publish(txCtx, event.PermissionChanged{BaseEvent: events.NewBaseEvent(event.PermissionChangedName), NodeID: request.NodeID, GrantID: grant.ID, Action: "CREATED"}); err != nil {
				return err
			}
		}
		if err := s.log(txCtx, request, grant.ID, "create", "ALLOW", ""); err != nil {
			return err
		}
		return s.complete(txCtx, request.CommandID, grant.ID)
	})
	return grant, err
}

var _ filesystemport.FilesystemGrantPort = (*GrantService)(nil)

func (s *GrantService) Revoke(ctx context.Context, request GrantRequest, grantID uuid.UUID) error {
	var err error
	request, err = s.authoritativeRequest(ctx, request)
	if err != nil {
		s.log(ctx, request, grantID, "revoke", "DENY", "tenant_mismatch")
		return domain.ErrResourceDenied
	}
	if request.CommandID == uuid.Nil {
		s.log(ctx, request, grantID, "revoke", "DENY", "idempotency_required")
		return domain.ErrIdempotencyConflict
	}
	if decision := s.authorizer.Authorize(ctx, Request{ActorID: request.ActorID, WorkspaceID: request.WorkspaceID, NamespaceID: request.NamespaceID, NodeID: request.NodeID, Permission: model.ResourcePermissionAdmin}); !decision.Allowed {
		s.log(ctx, request, grantID, "revoke", "DENY", decision.Reason)
		return domain.ErrResourceDenied
	}
	return s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		replayed, claimErr := s.claim(txCtx, request, "grant_revoke", grantRequestHash("grant_revoke", request, grantID), grantID)
		if claimErr != nil || replayed {
			return claimErr
		}
		if err := s.repo.RevokeResourceGrant(txCtx, request.NodeID, grantID); err != nil {
			return err
		}
		if s.bus != nil {
			if err := s.bus.Publish(txCtx, event.PermissionChanged{BaseEvent: events.NewBaseEvent(event.PermissionChangedName), NodeID: request.NodeID, GrantID: grantID, Action: "REVOKED"}); err != nil {
				return err
			}
		}
		if err := s.log(txCtx, request, grantID, "revoke", "ALLOW", ""); err != nil {
			return err
		}
		return s.complete(txCtx, request.CommandID, grantID)
	})
}

func (s *GrantService) List(ctx context.Context, request GrantRequest) ([]model.ResourceGrant, error) {
	var err error
	request, err = s.authoritativeRequest(ctx, request)
	if err != nil {
		return nil, domain.ErrResourceDenied
	}
	if decision := s.authorizer.Authorize(ctx, Request{ActorID: request.ActorID, WorkspaceID: request.WorkspaceID, NamespaceID: request.NamespaceID, NodeID: request.NodeID, Permission: model.ResourcePermissionAdmin}); !decision.Allowed {
		return nil, domain.ErrResourceDenied
	}
	return s.repo.ListNodeResourceGrants(ctx, request.NodeID)
}

func (s *GrantService) authoritativeRequest(ctx context.Context, request GrantRequest) (GrantRequest, error) {
	requestedOrganizationID := request.OrganizationID
	requestedNamespaceID := request.NamespaceID
	namespace, err := s.repo.GetNamespaceByWorkspace(ctx, request.WorkspaceID)
	if err != nil {
		request.OrganizationID = uuid.Nil
		request.NamespaceID = uuid.Nil
		return request, err
	}
	request.OrganizationID = namespace.OrganizationID
	request.NamespaceID = namespace.ID
	if requestedNamespaceID != namespace.ID || requestedOrganizationID != namespace.OrganizationID {
		return request, domain.ErrNamespaceMismatch
	}
	if _, err = s.repo.GetNode(ctx, namespace.ID, request.NodeID); err != nil {
		return request, err
	}
	return request, nil
}

func grantRequestHash(operation string, request GrantRequest, grantID uuid.UUID) string {
	grant := request.Grant
	value := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		operation, request.ActorID, request.OrganizationID, request.WorkspaceID, request.NamespaceID,
		request.NodeID, grantID, grant.PrincipalType, grant.PrincipalID, grant.Permission, grant.Effect)
	if grant.ExpiresAt != nil {
		value += "|" + grant.ExpiresAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *GrantService) claim(ctx context.Context, request GrantRequest, operation, requestHash string, grantID uuid.UUID) (bool, error) {
	if request.CommandID == uuid.Nil {
		return false, nil
	}
	claimed, err := s.repo.ClaimCommand(ctx, repository.CommandDedup{
		CommandID: request.CommandID, WorkspaceID: request.WorkspaceID, Operation: operation,
		RequestHash: requestHash, NodeID: &request.NodeID, RevisionID: &grantID,
	})
	if err != nil {
		return false, err
	}
	return claimed.State == "COMPLETED", nil
}

func (s *GrantService) complete(ctx context.Context, commandID, grantID uuid.UUID) error {
	if commandID == uuid.Nil {
		return nil
	}
	return s.repo.CompleteCommand(ctx, commandID, nil, &grantID)
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

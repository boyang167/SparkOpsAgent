package authorization

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

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
	if err := validateGrantOrganization(request); err != nil {
		s.log(ctx, request, uuid.Nil, "create", "DENY", "principal_organization_mismatch")
		return model.ResourceGrant{}, err
	}
	grant := request.Grant
	legacyGrantInput := grant.PrincipalType == "" && grant.PrincipalID == uuid.Nil && grant.GranteeID != uuid.Nil
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
	if legacyGrantInput {
		grant.InheritToChildren = true
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
	if err := validateGrantOrganization(request); err != nil {
		s.log(ctx, request, grantID, "revoke", "DENY", "principal_organization_mismatch")
		return err
	}
	grants, err := s.repo.ListNodeResourceGrants(ctx, request.NodeID)
	if err != nil {
		return err
	}
	found := false
	for _, grant := range grants {
		if grant.ID == grantID {
			request.Grant = grant
			found = true
			break
		}
	}
	if !found {
		if completed, commandErr := s.repo.GetCommand(ctx, request.CommandID); commandErr == nil &&
			completed.State == "COMPLETED" && completed.Operation == "grant_revoke" &&
			completed.RevisionID != nil && *completed.RevisionID == grantID {
			return nil
		}
		return domain.ErrGrantNotFound
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
	if err := validateGrantOrganization(request); err != nil {
		return nil, err
	}
	return s.repo.ListNodeResourceGrants(ctx, request.NodeID)
}

func validateGrantOrganization(request GrantRequest) error {
	if request.GranteeOrganizationID == uuid.Nil || request.GranteeOrganizationID != request.OrganizationID {
		return domain.ErrCrossOrganizationGrant
	}
	return nil
}

func (s *GrantService) authoritativeRequest(ctx context.Context, request GrantRequest) (GrantRequest, error) {
	requestedOrganizationID := request.OrganizationID
	requestedNamespaceID := request.NamespaceID
	namespace, err := s.repo.GetNamespace(ctx, requestedNamespaceID)
	if err != nil {
		request.OrganizationID = uuid.Nil
		request.NamespaceID = uuid.Nil
		return request, err
	}
	request.OrganizationID = namespace.OrganizationID
	request.NamespaceID = namespace.ID
	if namespace.NamespaceType == model.NamespaceTypeWorkspace && namespace.WorkspaceID != request.WorkspaceID {
		if authoritative, workspaceErr := s.repo.GetNamespaceByWorkspace(ctx, request.WorkspaceID); workspaceErr == nil {
			request.OrganizationID = authoritative.OrganizationID
			request.NamespaceID = authoritative.ID
		}
		return request, domain.ErrNamespaceMismatch
	}
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
	value := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%t",
		operation, request.ActorID, request.OrganizationID, request.WorkspaceID, request.NamespaceID,
		request.NodeID, grantID, grant.PrincipalType, grant.PrincipalID, grant.Permission, grant.Effect, grant.InheritToChildren)
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
	principal := map[string]any{
		"type": request.Grant.PrincipalType, "id": request.Grant.PrincipalID.String(),
		"permission": request.Grant.Permission, "effect": request.Grant.Effect,
	}
	detail := map[string]any{
		"workspace_id": request.WorkspaceID.String(), "namespace_id": request.NamespaceID.String(),
		"node_id": request.NodeID.String(), "principal_id": request.Grant.PrincipalID.String(),
		"decision": decision, "trace_id": request.TraceID, "grant_id": grantID.String(),
		"action":           strings.ToUpper(operation),
		"principal_before": principal, "principal_after": principal,
	}
	if operation == "create" {
		detail["principal_before"] = "NONE"
	}
	if operation == "revoke" {
		detail["principal_after"] = "REVOKED"
	}
	if reason != "" {
		detail["reason"] = reason
	}
	return s.audit.Log(ctx, audit.Entry{
		ActorID: kernel.MemberID{UUID: request.ActorID}, OrgID: &kernel.OrgID{UUID: request.OrganizationID},
		Action: "filesystem.grant." + operation, ResourceType: "filesystem", ResourceID: request.NodeID.String(), Detail: detail,
	})
}

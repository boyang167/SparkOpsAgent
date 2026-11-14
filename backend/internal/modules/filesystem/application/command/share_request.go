package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type PrincipalScope struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
}

type PrincipalScopeResolver interface {
	Resolve(context.Context, model.PrincipalType, uuid.UUID, uuid.UUID) (PrincipalScope, error)
}

type ShareAuthority interface {
	HasWorkspaceFileAdmin(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	HasOrganizationFileAdmin(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

type ShareRequestStore interface {
	GetNodeByID(context.Context, uuid.UUID) (*model.Node, error)
	GetNamespace(context.Context, uuid.UUID) (*model.Namespace, error)
	GetNodeClassification(context.Context, uuid.UUID) (model.TransferClassification, error)
	CreateShareRequest(context.Context, model.ShareRequest) error
	GetShareRequest(context.Context, uuid.UUID) (model.ShareRequest, error)
	ReviewShareRequestCAS(context.Context, model.ShareRequest, model.ShareRequestStatus) error
	CreateResourceGrants(context.Context, []model.ResourceGrant) error
	ClaimCommand(context.Context, repository.CommandDedup) (*repository.CommandDedup, error)
	CompleteCommand(context.Context, uuid.UUID, *uuid.UUID, *uuid.UUID) error
}

type SubmitShare struct {
	CommandID         uuid.UUID
	ActorID           uuid.UUID
	OrganizationID    uuid.UUID
	WorkspaceID       uuid.UUID
	NodeID            uuid.UUID
	PrincipalType     model.PrincipalType
	PrincipalIDs      []uuid.UUID
	Actions           []model.ResourcePermission
	InheritToChildren bool
	ExpiresAt         *time.Time
}

type ShareResult struct {
	Request model.ShareRequest
	Grants  []model.ResourceGrant
}

type ReviewShareRequest struct {
	CommandID       uuid.UUID
	ActorID         uuid.UUID
	OrganizationID  uuid.UUID
	RequestID       uuid.UUID
	RejectionReason string
}

type ShareRequestHandler struct {
	store     ShareRequestStore
	tx        postgres.Transactor
	bus       messaging.Bus
	scopes    PrincipalScopeResolver
	authority ShareAuthority
	now       func() time.Time
}

func NewShareRequestHandler(store ShareRequestStore, tx postgres.Transactor, bus messaging.Bus, scopes PrincipalScopeResolver, authority ShareAuthority) *ShareRequestHandler {
	return &ShareRequestHandler{store: store, tx: tx, bus: bus, scopes: scopes, authority: authority, now: func() time.Time { return time.Now().UTC() }}
}

func (h *ShareRequestHandler) Submit(ctx context.Context, cmd SubmitShare) (ShareResult, error) {
	var result ShareResult
	if err := validateSubmitShare(cmd); err != nil {
		return result, err
	}
	node, err := h.store.GetNodeByID(ctx, cmd.NodeID)
	if err != nil {
		return result, err
	}
	namespace, err := h.store.GetNamespace(ctx, node.NamespaceID)
	if err != nil {
		return result, err
	}
	if namespace.OrganizationID != cmd.OrganizationID {
		return result, domain.ErrResourceDenied
	}
	classification, err := h.store.GetNodeClassification(ctx, cmd.NodeID)
	if err != nil {
		return result, err
	}
	scopes, err := h.resolveScopes(ctx, cmd.PrincipalType, cmd.PrincipalIDs, namespace.OrganizationID)
	if err != nil {
		return result, err
	}
	immediate, err := h.sharePolicy(ctx, cmd, namespace, classification, scopes)
	if err != nil {
		return result, err
	}
	requestID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("filesystem-share-request:"+cmd.CommandID.String()))
	status := model.ShareRequestPending
	if immediate {
		status = model.ShareRequestApproved
	}
	request := model.ShareRequest{
		ID: requestID, OrganizationID: namespace.OrganizationID, NamespaceID: namespace.ID, NodeID: node.ID,
		RequestedBy: cmd.ActorID, PrincipalType: cmd.PrincipalType, PrincipalIDs: append([]uuid.UUID(nil), cmd.PrincipalIDs...),
		Actions: append([]model.ResourcePermission(nil), cmd.Actions...), InheritToChildren: cmd.InheritToChildren,
		ExpiresAt: cmd.ExpiresAt, Classification: classification, Status: status, CreatedAt: h.now(),
	}
	hash := shareHash("submit", request, cmd.ActorID, "")
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		replayed, claimErr := h.claim(txCtx, cmd.CommandID, cmd.WorkspaceID, node.ID, request.ID, hash)
		if claimErr != nil {
			return claimErr
		}
		if replayed {
			existing, getErr := h.store.GetShareRequest(txCtx, request.ID)
			result = ShareResult{Request: existing}
			if existing.Status == model.ShareRequestApproved {
				result.Grants = grantsForRequest(existing, cmd.CommandID)
			}
			return getErr
		}
		if err := h.store.CreateShareRequest(txCtx, request); err != nil {
			return err
		}
		result.Request = request
		if immediate {
			result.Grants = grantsForRequest(request, cmd.CommandID)
			if err := h.createGrantsAndEvents(txCtx, result.Grants); err != nil {
				return err
			}
		}
		return h.store.CompleteCommand(txCtx, cmd.CommandID, &node.ID, &request.ID)
	})
	return result, err
}

func (h *ShareRequestHandler) Approve(ctx context.Context, cmd ReviewShareRequest) (model.ShareRequest, error) {
	return h.review(ctx, cmd, true)
}

func (h *ShareRequestHandler) Reject(ctx context.Context, cmd ReviewShareRequest) (model.ShareRequest, error) {
	return h.review(ctx, cmd, false)
}

func (h *ShareRequestHandler) review(ctx context.Context, cmd ReviewShareRequest, approve bool) (model.ShareRequest, error) {
	if cmd.CommandID == uuid.Nil || cmd.ActorID == uuid.Nil || cmd.RequestID == uuid.Nil {
		return model.ShareRequest{}, domain.ErrIdempotencyConflict
	}
	request, err := h.store.GetShareRequest(ctx, cmd.RequestID)
	if err != nil {
		return model.ShareRequest{}, err
	}
	if cmd.OrganizationID != uuid.Nil && request.OrganizationID != cmd.OrganizationID {
		return model.ShareRequest{}, domain.ErrShareRequestNotFound
	}
	scopes, err := h.resolveScopes(ctx, request.PrincipalType, request.PrincipalIDs, request.OrganizationID)
	if err != nil {
		return model.ShareRequest{}, err
	}
	if approve && request.Classification == model.TransferClassificationConfidential && request.RequestedBy == cmd.ActorID {
		return model.ShareRequest{}, domain.ErrShareSelfApproval
	}
	if err := h.requireApprovalAuthority(ctx, cmd.ActorID, request.PrincipalType, request.OrganizationID, scopes); err != nil {
		return model.ShareRequest{}, err
	}
	operation := "reject"
	if approve {
		operation = "approve"
	}
	hash := reviewHash(operation, request.ID, cmd.ActorID, cmd.RejectionReason)
	var reviewed model.ShareRequest
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		replayed, claimErr := h.claim(txCtx, cmd.CommandID, firstWorkspace(scopes), request.NodeID, request.ID, hash)
		if claimErr != nil {
			return claimErr
		}
		if replayed {
			reviewed, err = h.store.GetShareRequest(txCtx, request.ID)
			return err
		}
		current, getErr := h.store.GetShareRequest(txCtx, request.ID)
		if getErr != nil {
			return getErr
		}
		if current.Status != model.ShareRequestPending {
			return domain.ErrShareRequestConflict
		}
		now := h.now()
		current.ReviewedBy = &cmd.ActorID
		current.ReviewedAt = &now
		if approve {
			current.Status = model.ShareRequestApproved
		} else {
			current.Status = model.ShareRequestRejected
			current.RejectionReason = cmd.RejectionReason
		}
		if err := h.store.ReviewShareRequestCAS(txCtx, current, model.ShareRequestPending); err != nil {
			return err
		}
		if approve {
			if err := h.createGrantsAndEvents(txCtx, grantsForRequest(current, cmd.CommandID)); err != nil {
				return err
			}
		}
		if err := h.store.CompleteCommand(txCtx, cmd.CommandID, &current.NodeID, &current.ID); err != nil {
			return err
		}
		reviewed = current
		return nil
	})
	return reviewed, err
}

func (h *ShareRequestHandler) resolveScopes(ctx context.Context, principalType model.PrincipalType, ids []uuid.UUID, organizationID uuid.UUID) ([]PrincipalScope, error) {
	if h.scopes == nil {
		return nil, domain.ErrUnsupportedPrincipal
	}
	scopes := make([]PrincipalScope, 0, len(ids))
	for _, id := range ids {
		scope, err := h.scopes.Resolve(ctx, principalType, id, organizationID)
		if err != nil {
			return nil, err
		}
		if scope.OrganizationID != organizationID {
			return nil, domain.ErrCrossOrganizationGrant
		}
		scopes = append(scopes, scope)
	}
	return scopes, nil
}

func (h *ShareRequestHandler) sharePolicy(ctx context.Context, cmd SubmitShare, namespace *model.Namespace, classification model.TransferClassification, scopes []PrincipalScope) (bool, error) {
	if cmd.PrincipalType == model.PrincipalOrganization {
		for _, action := range cmd.Actions {
			if action == model.ResourcePermissionDelete || action == model.ResourcePermissionShare || action == model.ResourcePermissionAdmin {
				return false, domain.ErrSharePolicyDenied
			}
		}
		if classification == model.TransferClassificationRestricted {
			return false, domain.ErrSharePolicyDenied
		}
	}
	if namespace.NamespaceType == model.NamespaceTypeOrganization {
		admin, err := h.authority.HasOrganizationFileAdmin(ctx, namespace.OrganizationID, cmd.ActorID)
		if err != nil {
			return false, err
		}
		if !admin || cmd.PrincipalType != model.PrincipalWorkspace {
			return false, domain.ErrResourceDenied
		}
		if classification == model.TransferClassificationConfidential {
			return false, nil
		}
		return true, nil
	}
	if classification == model.TransferClassificationConfidential {
		return false, nil
	}
	if cmd.PrincipalType == model.PrincipalUser && onlyReadActions(cmd.Actions) {
		return true, nil
	}
	switch cmd.PrincipalType {
	case model.PrincipalUser:
		return false, nil
	case model.PrincipalWorkspace, model.PrincipalRole:
		for _, scope := range scopes {
			admin, err := h.authority.HasWorkspaceFileAdmin(ctx, scope.WorkspaceID, cmd.ActorID)
			if err != nil {
				return false, err
			}
			if !admin {
				return false, nil
			}
		}
		return true, nil
	case model.PrincipalOrganization:
		admin, err := h.authority.HasOrganizationFileAdmin(ctx, namespace.OrganizationID, cmd.ActorID)
		return admin, err
	default:
		return false, domain.ErrUnsupportedPrincipal
	}
}

func (h *ShareRequestHandler) requireApprovalAuthority(ctx context.Context, actorID uuid.UUID, principalType model.PrincipalType, organizationID uuid.UUID, scopes []PrincipalScope) error {
	if principalType == model.PrincipalWorkspace || principalType == model.PrincipalRole {
		for _, scope := range scopes {
			allowed, err := h.authority.HasWorkspaceFileAdmin(ctx, scope.WorkspaceID, actorID)
			if err != nil {
				return err
			}
			if !allowed {
				return domain.ErrResourceDenied
			}
		}
		return nil
	}
	allowed, err := h.authority.HasOrganizationFileAdmin(ctx, organizationID, actorID)
	if err != nil {
		return err
	}
	if !allowed {
		return domain.ErrResourceDenied
	}
	return nil
}

func (h *ShareRequestHandler) createGrantsAndEvents(ctx context.Context, grants []model.ResourceGrant) error {
	if err := h.store.CreateResourceGrants(ctx, grants); err != nil {
		return err
	}
	if h.bus == nil {
		return nil
	}
	events := make([]sharedevents.Event, 0, len(grants))
	for _, grant := range grants {
		events = append(events, event.PermissionChanged{
			BaseEvent: sharedevents.NewBaseEvent(event.PermissionChangedName),
			NodeID:    grant.NodeID, GrantID: grant.ID, Action: "CREATED",
		})
	}
	return h.bus.Publish(ctx, events...)
}

func (h *ShareRequestHandler) claim(ctx context.Context, commandID, workspaceID, nodeID, requestID uuid.UUID, hash string) (bool, error) {
	claimed, err := h.store.ClaimCommand(ctx, repository.CommandDedup{
		CommandID: commandID, WorkspaceID: workspaceID, Operation: "share_request", RequestHash: hash,
		NodeID: &nodeID, RevisionID: &requestID,
	})
	if err != nil {
		return false, err
	}
	return claimed.State == "COMPLETED", nil
}

func validateSubmitShare(cmd SubmitShare) error {
	if cmd.CommandID == uuid.Nil || cmd.ActorID == uuid.Nil || cmd.OrganizationID == uuid.Nil || cmd.NodeID == uuid.Nil {
		return domain.ErrIdempotencyConflict
	}
	if len(cmd.PrincipalIDs) == 0 || len(cmd.Actions) == 0 {
		return domain.ErrSharePolicyDenied
	}
	switch cmd.PrincipalType {
	case model.PrincipalUser, model.PrincipalWorkspace, model.PrincipalOrganization, model.PrincipalRole:
	default:
		return domain.ErrUnsupportedPrincipal
	}
	seenPrincipals := map[uuid.UUID]struct{}{}
	for _, id := range cmd.PrincipalIDs {
		if id == uuid.Nil {
			return domain.ErrSharePolicyDenied
		}
		if _, ok := seenPrincipals[id]; ok {
			return domain.ErrSharePolicyDenied
		}
		seenPrincipals[id] = struct{}{}
	}
	seenActions := map[model.ResourcePermission]struct{}{}
	for _, action := range cmd.Actions {
		switch action {
		case model.ResourcePermissionRead, model.ResourcePermissionDownload, model.ResourcePermissionWrite,
			model.ResourcePermissionDelete, model.ResourcePermissionShare, model.ResourcePermissionAdmin:
		default:
			return domain.ErrSharePolicyDenied
		}
		if _, ok := seenActions[action]; ok {
			return domain.ErrSharePolicyDenied
		}
		seenActions[action] = struct{}{}
	}
	return nil
}

func onlyReadActions(actions []model.ResourcePermission) bool {
	for _, action := range actions {
		if action != model.ResourcePermissionRead && action != model.ResourcePermissionDownload {
			return false
		}
	}
	return true
}

func grantsForRequest(request model.ShareRequest, commandID uuid.UUID) []model.ResourceGrant {
	grants := make([]model.ResourceGrant, 0, len(request.PrincipalIDs)*len(request.Actions))
	for _, principalID := range request.PrincipalIDs {
		for _, action := range request.Actions {
			key := fmt.Sprintf("filesystem-share-grant:%s:%s:%s", commandID, principalID, action)
			grants = append(grants, model.ResourceGrant{
				ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(key)), NodeID: request.NodeID, OrganizationID: request.OrganizationID,
				GranteeID: principalID, PrincipalType: request.PrincipalType, PrincipalID: principalID,
				Permission: action, Effect: model.GrantEffectAllow, InheritToChildren: request.InheritToChildren,
				ExpiresAt: request.ExpiresAt, PolicyVersion: 1,
			})
		}
	}
	return grants
}

func shareHash(operation string, request model.ShareRequest, actorID uuid.UUID, reason string) string {
	payload, _ := json.Marshal(struct {
		Operation         string
		RequestID         uuid.UUID
		OrganizationID    uuid.UUID
		NamespaceID       uuid.UUID
		NodeID            uuid.UUID
		ActorID           uuid.UUID
		PrincipalType     model.PrincipalType
		PrincipalIDs      []uuid.UUID
		Actions           []model.ResourcePermission
		InheritToChildren bool
		ExpiresAt         *time.Time
		Reason            string
	}{
		operation, request.ID, request.OrganizationID, request.NamespaceID, request.NodeID, actorID,
		request.PrincipalType, request.PrincipalIDs, request.Actions, request.InheritToChildren, request.ExpiresAt, reason,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func reviewHash(operation string, requestID, actorID uuid.UUID, reason string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s", operation, requestID, actorID, reason)))
	return hex.EncodeToString(sum[:])
}

func firstWorkspace(scopes []PrincipalScope) uuid.UUID {
	for _, scope := range scopes {
		if scope.WorkspaceID != uuid.Nil {
			return scope.WorkspaceID
		}
	}
	return uuid.Nil
}

package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Authorization struct {
	Allowed     bool
	Reason      string
	TraceID     string
	WorkspaceID uuid.UUID
	ResourceID  uuid.UUID
	Permission  model.ResourcePermission
}

type authorizationError struct{ reason string }

func (e authorizationError) Error() string {
	if e.reason == "" {
		return "authorization denied"
	}
	return e.reason
}

func (authorizationError) Is(target error) bool { return target == domain.ErrResourceDenied }

func authorizationDenied(reason string) error { return authorizationError{reason: reason} }

func validateMutationNamespace(ns *model.Namespace, organizationID, workspaceID uuid.UUID) error {
	if ns == nil {
		return domain.ErrNamespaceNotFound
	}
	if ns.OrganizationID != organizationID {
		return domain.ErrNamespaceMismatch
	}
	if ns.NamespaceType == model.NamespaceTypeWorkspace && ns.WorkspaceID != workspaceID {
		return domain.ErrNamespaceMismatch
	}
	return nil
}

func RequireResourceAllowed(authorization Authorization, workspaceID, resourceID uuid.UUID, permission model.ResourcePermission) error {
	return requireAllowed(authorization, workspaceID, resourceID, permission)
}

func requireAllowed(authorization Authorization, workspaceID, resourceID uuid.UUID, permission model.ResourcePermission) error {
	if !authorization.Allowed {
		return authorizationDenied(authorization.Reason)
	}
	if authorization.WorkspaceID == workspaceID &&
		authorization.ResourceID == resourceID &&
		authorization.Permission == permission {
		return nil
	}
	return authorizationDenied("authorization_scope_mismatch")
}

type traceContextKey struct{}
type resourceContextKey struct{}
type auditDetailContextKey struct{}
type auditStateContextKey struct{}

type mutationAuditState struct {
	decision  string
	attempted bool
}

func withTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceContextKey{}, traceID)
}

func withResourceID(ctx context.Context, resourceID string) context.Context {
	return context.WithValue(ctx, resourceContextKey{}, resourceID)
}

func withAuditDetail(ctx context.Context, detail map[string]any) context.Context {
	return context.WithValue(ctx, auditDetailContextKey{}, detail)
}

type auditPort interface {
	Log(context.Context, audit.Entry) error
}

type auditService struct{ store audit.Store }

func requireMutationDependencies(repo repository.FilesystemRepository, tx postgres.Transactor) {
	if repo == nil || tx == nil {
		panic("filesystem mutation repository and transactor are required")
	}
}

func deniedWithAudit(ctx context.Context, logger auditPort, actor, organization uuid.UUID, workspace, action string, err error) error {
	return errors.Join(err, logMutation(ctx, logger, actor, organization, workspace, action, false, err))
}

func beginMutationAudit(ctx context.Context, logger auditPort, actor, organization uuid.UUID, workspace, action string) (context.Context, func(*error)) {
	state := &mutationAuditState{}
	ctx = context.WithValue(ctx, auditStateContextKey{}, state)
	return ctx, func(resultErr *error) {
		if resultErr != nil && *resultErr != nil {
			if !state.attempted || state.decision != "DENY" {
				*resultErr = errors.Join(*resultErr, logMutation(ctx, logger, actor, organization, workspace, action, false, *resultErr))
			}
			return
		}
		if !state.attempted {
			if auditErr := logMutation(ctx, logger, actor, organization, workspace, action, true, nil); resultErr != nil {
				*resultErr = auditErr
			}
		}
	}
}

func (s auditService) Log(ctx context.Context, entry audit.Entry) error {
	if s.store == nil {
		return nil
	}
	return s.store.Append(ctx, entry)
}

func logMutation(ctx context.Context, logger auditPort, actor, organization uuid.UUID, workspace, action string, allowed bool, reason error) error {
	if logger == nil {
		return nil
	}
	decision := "ALLOW"
	if !allowed {
		decision = "DENY"
	}
	detail := map[string]any{
		"workspace_id": workspace,
		"decision":     decision,
		"trace_id":     traceID(ctx),
	}
	if reason != nil {
		detail["reason"] = reasonCode(reason)
	}
	if extra, ok := ctx.Value(auditDetailContextKey{}).(map[string]any); ok {
		for key, value := range extra {
			detail[key] = value
		}
	}
	if state, ok := ctx.Value(auditStateContextKey{}).(*mutationAuditState); ok {
		state.attempted = true
		state.decision = decision
	}
	err := logger.Log(ctx, audit.Entry{
		ActorID:      kernel.MemberID{UUID: actor},
		Action:       action,
		ResourceType: "filesystem",
		ResourceID:   resourceID(ctx, workspace),
		OrgID:        &kernel.OrgID{UUID: organization},
		Detail:       detail,
	})
	return err
}

func traceID(ctx context.Context) string {
	value, _ := ctx.Value(traceContextKey{}).(string)
	return value
}

func resourceID(ctx context.Context, fallback string) string {
	if value, ok := ctx.Value(resourceContextKey{}).(string); ok && value != "" {
		return value
	}
	return fallback
}

func reasonCode(err error) string {
	switch {
	case errors.Is(err, domain.ErrResourceDenied):
		var denied authorizationError
		if errors.As(err, &denied) && denied.reason != "" {
			return denied.reason
		}
		return "resource_denied"
	case errors.Is(err, domain.ErrQuotaExceeded):
		return "quota_exceeded"
	case errors.Is(err, domain.ErrNodeNameConflict):
		return "name_conflict"
	case errors.Is(err, domain.ErrNamespaceMismatch):
		return "namespace_mismatch"
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return "idempotency_conflict"
	case errors.Is(err, domain.ErrTransferIdempotencyConflict):
		return "transfer_idempotency_conflict"
	case errors.Is(err, domain.ErrParentNotFound):
		return "parent_not_found"
	case errors.Is(err, domain.ErrParentNotDirectory):
		return "parent_not_directory"
	case errors.Is(err, model.ErrInvalidTransferSize):
		return "invalid_transfer_size"
	case errors.Is(err, model.ErrInvalidTransferName):
		return "invalid_transfer_name"
	case errors.Is(err, model.ErrInvalidTransferChecksum):
		return "invalid_transfer_checksum"
	case errors.Is(err, model.ErrInvalidTransferProtocol):
		return "invalid_transfer_protocol"
	case errors.Is(err, model.ErrInvalidTransferExpiry):
		return "invalid_transfer_expiry"
	case errors.Is(err, model.ErrInvalidNodeName):
		return "invalid_name"
	case errors.Is(err, domain.ErrOutcomeUnknown):
		return "outcome_unknown"
	default:
		return "operation_failed"
	}
}

func mutationRequestHash(operation string, values ...any) string {
	var value strings.Builder
	value.WriteString(operation)
	for _, item := range values {
		part := fmt.Sprint(item)
		fmt.Fprintf(&value, "|%d:%s", len(part), part)
	}
	sum := sha256.Sum256([]byte(value.String()))
	return hex.EncodeToString(sum[:])
}

func claimMutation(
	ctx context.Context,
	repo repository.FilesystemRepository,
	commandID, workspaceID uuid.UUID,
	operation, requestHash string,
	nodeID, revisionID *uuid.UUID,
) (bool, *repository.CommandDedup, error) {
	if commandID == uuid.Nil {
		return false, nil, nil
	}
	claimed, err := repo.ClaimCommand(ctx, repository.CommandDedup{
		CommandID: commandID, WorkspaceID: workspaceID, Operation: operation, RequestHash: requestHash,
		NodeID: nodeID, RevisionID: revisionID,
	})
	if err != nil {
		return false, nil, err
	}
	if claimed.State == "COMPLETED" {
		markMutationReplay(ctx)
		return true, claimed, nil
	}
	return false, claimed, nil
}

func markMutationReplay(ctx context.Context) {
	if state, ok := ctx.Value(auditStateContextKey{}).(*mutationAuditState); ok {
		state.attempted = true
		state.decision = "ALLOW"
	}
}

func completeMutation(ctx context.Context, repo repository.FilesystemRepository, commandID uuid.UUID, nodeID, revisionID *uuid.UUID) error {
	if commandID == uuid.Nil {
		return nil
	}
	return repo.CompleteCommand(ctx, commandID, nodeID, revisionID)
}

const idempotentMutationAttempts = 8

func mutationRetryBackoff(attempt int) time.Duration {
	base := time.Millisecond << attempt
	return base + time.Duration(rand.Int64N(int64(base)))
}

func waitForMutationRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func withinIdempotentMutation(ctx context.Context, tx postgres.Transactor, mutate func(context.Context) error) error {
	return withinIdempotentMutationWithRetry(ctx, tx, mutate, mutationRetryBackoff, waitForMutationRetry)
}

func withinIdempotentMutationWithRetry(
	ctx context.Context,
	tx postgres.Transactor,
	mutate func(context.Context) error,
	retryBackoff func(int) time.Duration,
	retryWait func(context.Context, time.Duration) error,
) error {
	for attempt := 0; attempt < idempotentMutationAttempts; attempt++ {
		err := tx.WithinTransaction(ctx, mutate)
		if !errors.Is(err, domain.ErrTransactionRetry) && !errors.Is(err, domain.ErrCommandInProgress) {
			return err
		}
		if attempt == idempotentMutationAttempts-1 {
			return domain.ErrTransactionExhausted
		}
		if err := retryWait(ctx, retryBackoff(attempt)); err != nil {
			return err
		}
	}
	return domain.ErrTransactionExhausted
}

type CreateNamespace struct {
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	Authorization
}

type CreateNamespaceHandler struct {
	repo              repository.FilesystemRepository
	tx                postgres.Transactor
	bus               messaging.Bus
	audit             auditPort
	localStorageRoot  string
	ensureDefaultProv *EnsureDefaultLocalProviderHandler
}

func NewCreateNamespaceHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *CreateNamespaceHandler {
	requireMutationDependencies(repo, tx)
	return &CreateNamespaceHandler{repo: repo, tx: tx, bus: bus, audit: logger}
}

func (h *CreateNamespaceHandler) WithDefaultLocalProvider(root string, ensure *EnsureDefaultLocalProviderHandler) *CreateNamespaceHandler {
	h.localStorageRoot = strings.TrimSpace(root)
	h.ensureDefaultProv = ensure
	return h
}

func (h *CreateNamespaceHandler) Handle(ctx context.Context, cmd CreateNamespace) (out *model.Namespace, err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.namespace.create")
	defer finishAudit(&err)
	if !cmd.Allowed {
		err := authorizationDenied(cmd.Reason)
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.namespace.create", err)
	}
	if existing, lookupErr := h.repo.GetNamespaceByWorkspace(ctx, cmd.WorkspaceID); lookupErr == nil {
		if existing.OrganizationID != cmd.OrganizationID {
			return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.namespace.create", domain.ErrNamespaceMismatch)
		}
		return existing, nil
	} else if !errors.Is(lookupErr, domain.ErrNamespaceNotFound) {
		return nil, lookupErr
	}
	ns := &model.Namespace{ID: uuid.New(), NamespaceType: model.NamespaceTypeWorkspace, OrganizationID: cmd.OrganizationID, WorkspaceID: cmd.WorkspaceID, Status: "ACTIVE"}
	ctx = withResourceID(ctx, ns.ID.String())
	root, err := model.NewRootDirectory(ns.ID, cmd.ActorID, ".root")
	if err != nil {
		return nil, err
	}
	err = withinIdempotentMutation(ctx, h.tx, func(txCtx context.Context) error {
		if err := h.repo.CreateNamespace(txCtx, ns); err != nil {
			return err
		}
		if err := h.repo.CreateNode(txCtx, root); err != nil {
			return err
		}
		ns.RootNodeID = &root.ID
		if err := h.repo.SetNamespaceRoot(txCtx, ns.WorkspaceID, root.ID); err != nil {
			return err
		}
		if h.bus != nil {
			ev := event.FileCreated{BaseEvent: events.NewBaseEvent(event.FileCreatedName), FileID: root.ID, NamespaceID: ns.ID}
			if err := h.bus.Publish(txCtx, ev); err != nil {
				return err
			}
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.namespace.create", true, nil)
	})
	if err != nil {
		if existing, readErr := h.repo.GetNamespaceByWorkspace(ctx, cmd.WorkspaceID); readErr == nil {
			if existing.OrganizationID == cmd.OrganizationID {
				return existing, nil
			}
			return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.namespace.create", domain.ErrNamespaceMismatch)
		}
		return nil, err
	}
	if h.ensureDefaultProv != nil && h.localStorageRoot != "" {
		if _, ensureErr := h.ensureDefaultProv.Handle(ctx, EnsureDefaultLocalProvider{
			OrganizationID: cmd.OrganizationID,
			StorageRoot:    h.localStorageRoot,
		}); ensureErr != nil {
			return nil, ensureErr
		}
	}
	return ns, nil
}

type CreateDirectory struct {
	CommandID      uuid.UUID
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	ParentID       *uuid.UUID
	Name           string
	Authorization
}

type CreateDirectoryHandler struct {
	repo  repository.FilesystemRepository
	tx    postgres.Transactor
	bus   messaging.Bus
	audit auditPort
}

func NewCreateDirectoryHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *CreateDirectoryHandler {
	requireMutationDependencies(repo, tx)
	return &CreateDirectoryHandler{repo: repo, tx: tx, bus: bus, audit: logger}
}

func (h *CreateDirectoryHandler) Handle(ctx context.Context, cmd CreateDirectory) (out *model.Node, err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ns, scopeErr := h.repo.GetNamespace(ctx, cmd.NamespaceID)
	parent := uuid.Nil
	if cmd.ParentID != nil {
		parent = *cmd.ParentID
	} else if scopeErr == nil && ns != nil && ns.RootNodeID != nil {
		parent = *ns.RootNodeID
	}
	if parent != uuid.Nil {
		ctx = withResourceID(ctx, parent.String())
	}
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.directory.create")
	defer finishAudit(&err)
	if scopeErr != nil {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.directory.create", scopeErr)
	}
	if !cmd.Allowed {
		err := authorizationDenied(cmd.Reason)
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.directory.create", err)
	}
	if ns.OrganizationID != cmd.OrganizationID ||
		(ns.NamespaceType == model.NamespaceTypeWorkspace && ns.WorkspaceID != cmd.WorkspaceID) {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.directory.create", domain.ErrNamespaceMismatch)
	}
	node, err := model.NewDirectory(cmd.NamespaceID, cmd.ActorID, cmd.Name, parent)
	if err != nil {
		return nil, err
	}
	if cmd.CommandID != uuid.Nil {
		node.ID = uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-directory:"+cmd.CommandID.String()))
	}
	requestHash := mutationRequestHash("create_directory", cmd.WorkspaceID, cmd.NamespaceID, parent, node.Name, cmd.ActorID)
	ctx = withResourceID(ctx, node.ID.String())
	err = withinIdempotentMutation(ctx, h.tx, func(txCtx context.Context) error {
		completed, claimed, err := claimMutation(txCtx, h.repo, cmd.CommandID, cmd.WorkspaceID, "create_directory", requestHash, &node.ID, nil)
		if err != nil {
			return err
		}
		if completed {
			if claimed.NodeID == nil {
				return domain.ErrOutcomeUnknown
			}
			node, err = h.repo.GetNode(txCtx, cmd.NamespaceID, *claimed.NodeID)
			return err
		}
		if err := h.repo.CreateNode(txCtx, node); err != nil {
			return err
		}
		if h.bus != nil {
			if err := h.bus.Publish(txCtx, event.DirectoryCreated{
				BaseEvent: events.NewBaseEvent(event.DirectoryCreatedName), NodeID: node.ID,
				NamespaceID: node.NamespaceID, ParentID: parent,
			}); err != nil {
				return err
			}
		}
		if err := completeMutation(txCtx, h.repo, cmd.CommandID, &node.ID, nil); err != nil {
			return err
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.directory.create", true, nil)
	})
	return node, err
}

type PutLocalFile struct {
	CommandID         uuid.UUID
	ActorID           uuid.UUID
	OrganizationID    uuid.UUID
	WorkspaceID       uuid.UUID
	NamespaceID       uuid.UUID
	ParentID          *uuid.UUID
	Name              string
	MIMEType          string
	Size              int64
	SHA256            string
	Body              io.Reader
	ReplicaProviderID uuid.UUID
	NodeID            *uuid.UUID
	Authorization
}

type PutLocalFileHandler struct {
	repo  repository.FilesystemRepository
	store storage.Provider
	tx    postgres.Transactor
	bus   messaging.Bus
	audit auditPort
}

func putRequestHash(cmd PutLocalFile, parent uuid.UUID) string {
	target := ""
	if cmd.NodeID != nil {
		target = cmd.NodeID.String()
	}
	value := fmt.Sprintf("put|%s|%s|%s|%s|%s|%s|%s|%s|%d|%s|%s|%s", cmd.OrganizationID, cmd.WorkspaceID, cmd.NamespaceID, target, parent, cmd.Name, cmd.ActorID, cmd.ReplicaProviderID, cmd.Size, cmd.MIMEType, strings.ToLower(strings.TrimSpace(cmd.SHA256)), cmd.CommandID)
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func NewPutLocalFileHandler(repo repository.FilesystemRepository, store storage.Provider, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *PutLocalFileHandler {
	requireMutationDependencies(repo, tx)
	if store == nil {
		panic("filesystem local storage provider is required")
	}
	return &PutLocalFileHandler{repo: repo, store: store, tx: tx, bus: bus, audit: logger}
}

func (h *PutLocalFileHandler) Handle(ctx context.Context, cmd PutLocalFile) (out *model.Node, err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local")
	defer finishAudit(&err)
	if !cmd.Allowed {
		err := authorizationDenied(cmd.Reason)
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", err)
	}
	if cmd.CommandID == uuid.Nil {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", domain.ErrIdempotencyConflict)
	}
	ns, err := h.repo.GetNamespaceByWorkspace(ctx, cmd.WorkspaceID)
	if err != nil {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", err)
	}
	if ns.ID != cmd.NamespaceID || ns.OrganizationID != cmd.OrganizationID {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", domain.ErrNamespaceMismatch)
	}
	parent := uuid.Nil
	if cmd.ParentID != nil {
		parent = *cmd.ParentID
	} else if ns.RootNodeID != nil {
		parent = *ns.RootNodeID
	}
	if len(cmd.SHA256) != 64 {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", errors.New("invalid checksum"))
	}
	requestHash := putRequestHash(cmd, parent)
	if prior, priorErr := h.repo.GetCommand(ctx, cmd.CommandID); priorErr == nil {
		if prior.RequestHash != requestHash {
			return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", domain.ErrIdempotencyConflict)
		}
		if prior.State != "COMPLETED" {
			return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", domain.ErrCommandInProgress)
		}
		if prior.NodeID == nil || prior.RevisionID == nil {
			return nil, domain.ErrOutcomeUnknown
		}
		completedNode, nodeErr := h.repo.GetNode(ctx, cmd.NamespaceID, *prior.NodeID)
		if nodeErr != nil {
			return nil, nodeErr
		}
		if _, revisionErr := h.repo.GetRevision(ctx, completedNode.ID, *prior.RevisionID); revisionErr != nil {
			return nil, revisionErr
		}
		return completedNode, nil
	} else if !errors.Is(priorErr, domain.ErrCommandNotFound) {
		return nil, priorErr
	}
	nodeID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-node:"+cmd.CommandID.String()))
	if cmd.NodeID != nil {
		nodeID = *cmd.NodeID
	}
	node, err := model.NewFile(cmd.NamespaceID, cmd.ActorID, cmd.Name, parent)
	if err != nil {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", err)
	}
	node.ID = nodeID
	ctx = withResourceID(ctx, node.ID.String())
	existing, existingErr := h.repo.GetNode(ctx, cmd.NamespaceID, node.ID)
	deterministicRevisionID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-revision:"+cmd.CommandID.String()))
	if existingErr == nil {
		if prior, priorErr := h.repo.GetRevision(ctx, existing.ID, deterministicRevisionID); priorErr == nil {
			existingBlob, blobErr := h.repo.GetBlob(ctx, prior.BlobID())
			existingReplica, replicaErr := h.repo.GetAvailableReplica(ctx, prior.BlobID(), cmd.ReplicaProviderID)
			if existing.NamespaceID != cmd.NamespaceID ||
				existing.Type != model.NodeTypeFile ||
				existing.Name != node.Name ||
				existing.NormalizedName != node.NormalizedName ||
				!sameUUIDPointer(existing.ParentID, node.ParentID) ||
				existing.OwnerID != cmd.ActorID ||
				prior.Size() != cmd.Size ||
				prior.MIMEType() != cmd.MIMEType ||
				prior.CreatedBy() != cmd.ActorID ||
				blobErr != nil ||
				existingBlob.DedupDomain != cmd.OrganizationID ||
				existingBlob.Size != cmd.Size ||
				!strings.EqualFold(existingBlob.SHA256, cmd.SHA256) ||
				replicaErr != nil ||
				existingReplica.ProviderID != cmd.ReplicaProviderID ||
				!strings.EqualFold(existingReplica.Checksum, cmd.SHA256) {
				return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", domain.ErrIdempotencyConflict)
			}
			rebuildErr := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
				claimed, claimErr := h.repo.ClaimCommand(txCtx, repository.CommandDedup{
					CommandID: cmd.CommandID, WorkspaceID: cmd.WorkspaceID, Operation: "put_local",
					RequestHash: requestHash, NodeID: &existing.ID, RevisionID: &deterministicRevisionID,
				})
				if claimErr != nil {
					return claimErr
				}
				if claimed.State == "COMPLETED" {
					return nil
				}
				return h.repo.CompleteCommand(txCtx, cmd.CommandID, &existing.ID, &deterministicRevisionID)
			})
			if rebuildErr != nil {
				return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", rebuildErr)
			}
			return existing, nil
		}
	}
	if cmd.NodeID == nil && existingErr == nil && existing.CurrentRevisionID != nil {
		existingRevision, revisionErr := h.repo.GetRevision(ctx, existing.ID, *existing.CurrentRevisionID)
		if revisionErr == nil && existingRevision.Size() == cmd.Size && existingRevision.MIMEType() == cmd.MIMEType {
			if cmd.SHA256 == "" {
				return existing, nil
			}
			existingBlob, blobErr := h.repo.GetBlob(ctx, existingRevision.BlobID())
			if blobErr == nil && strings.EqualFold(existingBlob.SHA256, cmd.SHA256) {
				return existing, nil
			}
		}
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", domain.ErrIdempotencyConflict)
	}
	if existingErr != nil && !errors.Is(existingErr, domain.ErrNodeNotFound) {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", existingErr)
	}
	objectKey := fmt.Sprintf("sessions/%s/content", cmd.CommandID)
	info, err := h.store.Put(ctx, storage.PutRequest{ObjectKey: objectKey, Body: cmd.Body, Size: cmd.Size, SHA256: cmd.SHA256})
	if err != nil {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", err)
	}
	deleteObject := true
	defer func() {
		if deleteObject {
			_ = h.store.Delete(context.Background(), info.ObjectKey)
		}
	}()
	revisionID := deterministicRevisionID
	providerID := cmd.ReplicaProviderID
	if providerID == uuid.Nil {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", errors.New("replica provider id is required"))
	}
	var completedNodeID *uuid.UUID
	var usedUploadedObject bool
	for attempt := 0; attempt < 5; attempt++ {
		err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
			claimed, claimErr := h.repo.ClaimCommand(txCtx, repository.CommandDedup{CommandID: cmd.CommandID, WorkspaceID: cmd.WorkspaceID, Operation: "put_local", RequestHash: requestHash, NodeID: &node.ID, RevisionID: &revisionID})
			if claimErr != nil {
				return claimErr
			}
			if claimed.State == "COMPLETED" {
				if claimed.NodeID == nil || claimed.RevisionID == nil {
					return domain.ErrOutcomeUnknown
				}
				completedNodeID = claimed.NodeID
				return nil
			}
			if existing == nil {
				if err := h.repo.CreateNode(txCtx, node); err != nil {
					return err
				}
			}
			revisionNo := 1
			if existing != nil && cmd.NodeID != nil {
				var allocateErr error
				revisionNo, allocateErr = h.repo.NextRevisionNumber(txCtx, node.ID)
				if allocateErr != nil {
					return allocateErr
				}
			}
			blob, acquireErr := h.repo.AcquireBlob(txCtx, cmd.OrganizationID, info.SHA256, info.Size)
			if acquireErr != nil {
				return acquireErr
			}
			replica, replicaErr := h.repo.GetAvailableReplica(txCtx, blob.ID, uuid.Nil)
			if replicaErr != nil && !errors.Is(replicaErr, domain.ErrReplicaConflict) {
				return replicaErr
			}
			if errors.Is(replicaErr, domain.ErrReplicaConflict) {
				verifiedAt := time.Now().UTC()
				replica = &model.StorageReplica{
					ID:     uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-replica:"+cmd.CommandID.String())),
					BlobID: blob.ID, ProviderID: providerID, ObjectKey: info.ObjectKey, Checksum: info.SHA256,
					State: model.ReplicaStateAvailable, VerifiedAt: &verifiedAt,
				}
				usedUploadedObject = true
			} else {
				replica = nil
				usedUploadedObject = false
			}
			revision, revisionErr := model.NewRevisionWithID(revisionID, node.ID, revisionNo, blob.ID, info.Size, cmd.MIMEType, cmd.ActorID)
			if revisionErr != nil {
				return revisionErr
			}
			if err := h.repo.SaveRevision(txCtx, revision, blob, replica); err != nil {
				return err
			}
			if err := h.repo.SetCurrentRevision(txCtx, node.ID, revision.ID()); err != nil {
				return err
			}
			filesDelta := int64(0)
			if existing == nil {
				filesDelta = 1
			}
			if err := h.repo.ApplyQuotaUsage(txCtx, cmd.WorkspaceID, info.Size, filesDelta); err != nil {
				return err
			}
			if h.bus != nil {
				if err := h.bus.Publish(txCtx,
					event.FileCreated{BaseEvent: events.NewBaseEvent(event.FileCreatedName), FileID: node.ID, NamespaceID: node.NamespaceID},
					event.FileRevisionCreated{BaseEvent: events.NewBaseEvent(event.FileRevisionCreatedName), FileID: node.ID, RevisionID: revision.ID(), RevisionNo: revision.RevisionNo()},
				); err != nil {
					return err
				}
			}
			if err := h.repo.CompleteCommand(txCtx, cmd.CommandID, &node.ID, &revisionID); err != nil {
				return err
			}
			return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", true, nil)
		})
		if !errors.Is(err, domain.ErrTransactionRetry) {
			break
		}
	}
	if err != nil {
		if errors.Is(err, postgres.ErrCommitOutcomeUnknown) || errors.Is(err, storage.ErrOutcomeUnknown) {
			authoritative, readErr := h.repo.GetCommand(ctx, cmd.CommandID)
			if readErr == nil &&
				authoritative.State == "COMPLETED" &&
				authoritative.RequestHash == requestHash &&
				authoritative.NodeID != nil &&
				authoritative.RevisionID != nil {
				result, resultErr := h.repo.GetNode(ctx, cmd.NamespaceID, *authoritative.NodeID)
				if resultErr == nil {
					if _, revisionErr := h.repo.GetRevision(ctx, result.ID, *authoritative.RevisionID); revisionErr == nil {
						deleteObject = !usedUploadedObject
						return result, nil
					}
				}
			}
			if readErr == nil || !errors.Is(readErr, domain.ErrCommandNotFound) {
				deleteObject = false
			}
			outcomeErr := fmt.Errorf("%w: commit result could not be confirmed", domain.ErrOutcomeUnknown)
			return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", outcomeErr)
		}
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", err)
	}
	deleteObject = !usedUploadedObject
	if completedNodeID != nil {
		return h.repo.GetNode(ctx, cmd.NamespaceID, *completedNodeID)
	}
	node.CurrentRevisionID = &revisionID
	return node, nil
}

func sameUUIDPointer(left, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

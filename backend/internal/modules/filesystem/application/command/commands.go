package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

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
	Allowed bool
	Reason  string
	TraceID string
}

type traceContextKey struct{}
type resourceContextKey struct{}

func withTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceContextKey{}, traceID)
}

func withResourceID(ctx context.Context, resourceID string) context.Context {
	return context.WithValue(ctx, resourceContextKey{}, resourceID)
}

type auditPort interface {
	Log(context.Context, audit.Entry) error
}

type auditService struct{ store audit.Store }

func deniedWithAudit(ctx context.Context, logger auditPort, actor, organization uuid.UUID, workspace, action string, err error) error {
	return errors.Join(err, logMutation(ctx, logger, actor, organization, workspace, action, false, err))
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
	return logger.Log(ctx, audit.Entry{
		ActorID:      kernel.MemberID{UUID: actor},
		Action:       action,
		ResourceType: "filesystem",
		ResourceID:   resourceID(ctx, workspace),
		OrgID:        &kernel.OrgID{UUID: organization},
		Detail:       detail,
	})
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
	case errors.Is(err, domain.ErrQuotaExceeded):
		return "quota_exceeded"
	case errors.Is(err, domain.ErrNodeNameConflict):
		return "name_conflict"
	case errors.Is(err, domain.ErrNamespaceMismatch):
		return "namespace_mismatch"
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return "idempotency_conflict"
	case errors.Is(err, model.ErrInvalidNodeName):
		return "invalid_name"
	case errors.Is(err, domain.ErrOutcomeUnknown):
		return "outcome_unknown"
	default:
		return "operation_failed"
	}
}

type CreateNamespace struct {
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	Authorization
}

type CreateNamespaceHandler struct {
	repo  repository.FilesystemRepository
	tx    postgres.Transactor
	bus   messaging.Bus
	audit auditPort
}

func NewCreateNamespaceHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *CreateNamespaceHandler {
	return &CreateNamespaceHandler{repo: repo, tx: tx, bus: bus, audit: logger}
}

func (h *CreateNamespaceHandler) Handle(ctx context.Context, cmd CreateNamespace) (*model.Namespace, error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.namespace.create", err)
	}
	if existing, err := h.repo.GetNamespaceByWorkspace(ctx, cmd.WorkspaceID); err == nil {
		if existing.OrganizationID != cmd.OrganizationID {
			return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.namespace.create", domain.ErrNamespaceMismatch)
		}
		return existing, nil
	}
	ns := &model.Namespace{ID: uuid.New(), OrganizationID: cmd.OrganizationID, WorkspaceID: cmd.WorkspaceID, Status: "ACTIVE"}
	ctx = withResourceID(ctx, ns.ID.String())
	root, err := model.NewRootDirectory(ns.ID, cmd.ActorID, ".root")
	if err != nil {
		return nil, err
	}
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
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
	return ns, nil
}

type CreateDirectory struct {
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
	return &CreateDirectoryHandler{repo: repo, tx: tx, bus: bus, audit: logger}
}

func (h *CreateDirectoryHandler) Handle(ctx context.Context, cmd CreateDirectory) (*model.Node, error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.directory.create", err)
	}
	ns, err := h.repo.GetNamespaceByWorkspace(ctx, cmd.WorkspaceID)
	if err != nil {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.directory.create", err)
	}
	if ns.ID != cmd.NamespaceID {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.directory.create", domain.ErrNamespaceMismatch)
	}
	parent := uuid.Nil
	if cmd.ParentID != nil {
		parent = *cmd.ParentID
	} else if ns.RootNodeID != nil {
		parent = *ns.RootNodeID
	}
	node, err := model.NewDirectory(cmd.NamespaceID, cmd.ActorID, cmd.Name, parent)
	if err != nil {
		return nil, err
	}
	ctx = withResourceID(ctx, node.ID.String())
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.CreateNode(txCtx, node); err != nil {
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
	value := fmt.Sprintf("put|%s|%s|%s|%s|%s|%s|%s|%d|%s|%s|%s", cmd.WorkspaceID, cmd.NamespaceID, target, parent, cmd.Name, cmd.ActorID, cmd.ReplicaProviderID, cmd.Size, cmd.MIMEType, strings.ToLower(strings.TrimSpace(cmd.SHA256)), cmd.CommandID)
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func NewPutLocalFileHandler(repo repository.FilesystemRepository, store storage.Provider, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *PutLocalFileHandler {
	return &PutLocalFileHandler{repo: repo, store: store, tx: tx, bus: bus, audit: logger}
}

func (h *PutLocalFileHandler) Handle(ctx context.Context, cmd PutLocalFile) (*model.Node, error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", err)
	}
	if h.store == nil || h.tx == nil {
		return nil, errors.New("filesystem local file dependencies are required")
	}
	if cmd.CommandID == uuid.Nil {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", domain.ErrIdempotencyConflict)
	}
	ns, err := h.repo.GetNamespaceByWorkspace(ctx, cmd.WorkspaceID)
	if err != nil {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", err)
	}
	if ns.ID != cmd.NamespaceID {
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
		if prior.NodeID != nil {
			return h.repo.GetNode(ctx, cmd.NamespaceID, *prior.NodeID)
		}
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
			if prior.Size() == cmd.Size && prior.MIMEType() == cmd.MIMEType {
				return existing, nil
			}
			return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", domain.ErrIdempotencyConflict)
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
	published := true
	defer func() {
		if published {
			_ = h.store.Delete(context.Background(), info.ObjectKey)
		}
	}()
	blob := &model.Blob{ID: uuid.NewSHA1(uuid.NameSpaceURL, []byte(cmd.CommandID.String())), DedupDomain: cmd.OrganizationID, SHA256: info.SHA256, Size: info.Size, RefCount: 1, State: model.BlobStateAvailable}
	revisionID := deterministicRevisionID
	revisionNo := 1
	if existing == nil && cmd.NodeID != nil {
		revisions, _, listErr := h.repo.ListRevisions(ctx, node.ID, cmd.NamespaceID, 1, "")
		if listErr == nil && len(revisions) > 0 {
			revisionNo = revisions[0].RevisionNo() + 1
			revisionID = uuid.New()
		}
	}
	revision, err := model.NewRevisionWithID(revisionID, node.ID, revisionNo, blob.ID, info.Size, cmd.MIMEType, cmd.ActorID)
	if err != nil {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", err)
	}
	providerID := cmd.ReplicaProviderID
	if providerID == uuid.Nil {
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", errors.New("replica provider id is required"))
	}
	replica := &model.StorageReplica{ID: uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-replica:"+cmd.CommandID.String())), BlobID: blob.ID, ProviderID: providerID, ObjectKey: info.ObjectKey, Checksum: info.SHA256, State: model.ReplicaStateAvailable}
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		claimed, claimErr := h.repo.ClaimCommand(txCtx, repository.CommandDedup{CommandID: cmd.CommandID, WorkspaceID: cmd.WorkspaceID, Operation: "put_local", RequestHash: requestHash, NodeID: &node.ID, RevisionID: &revisionID})
		if claimErr != nil {
			return claimErr
		}
		if claimed.State == "COMPLETED" {
			return domain.ErrCommandInProgress
		}
		if existing == nil {
			if err := h.repo.CreateNode(txCtx, node); err != nil {
				return err
			}
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
	if err != nil {
		if errors.Is(err, storage.ErrOutcomeUnknown) {
			published = false
			return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", domain.ErrOutcomeUnknown)
		}
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", err)
	}
	published = false
	return node, nil
}

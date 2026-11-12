package command

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"

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
		"trace_id":     "",
	}
	if reason != nil {
		detail["reason"] = reason.Error()
	}
	return logger.Log(ctx, audit.Entry{
		ActorID:      kernel.MemberID{UUID: actor},
		Action:       action,
		ResourceType: "filesystem",
		ResourceID:   workspace,
		OrgID:        &kernel.OrgID{UUID: organization},
		Detail:       detail,
	})
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
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.namespace.create", err)
	}
	if existing, err := h.repo.GetNamespaceByWorkspace(ctx, cmd.WorkspaceID); err == nil {
		return existing, nil
	}
	ns := &model.Namespace{ID: uuid.New(), OrganizationID: cmd.OrganizationID, WorkspaceID: cmd.WorkspaceID, Status: "ACTIVE"}
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
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.directory.create", err)
	}
	parent := uuid.Nil
	if cmd.ParentID != nil {
		parent = *cmd.ParentID
	}
	node, err := model.NewDirectory(cmd.NamespaceID, cmd.ActorID, cmd.Name, parent)
	if err != nil {
		return nil, err
	}
	if cmd.ParentID == nil {
		node.ParentID = nil
	}
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
	Authorization
}

type PutLocalFileHandler struct {
	repo  repository.FilesystemRepository
	store storage.Provider
	tx    postgres.Transactor
	bus   messaging.Bus
	audit auditPort
}

func NewPutLocalFileHandler(repo repository.FilesystemRepository, store storage.Provider, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *PutLocalFileHandler {
	return &PutLocalFileHandler{repo: repo, store: store, tx: tx, bus: bus, audit: logger}
}

func (h *PutLocalFileHandler) Handle(ctx context.Context, cmd PutLocalFile) (*model.Node, error) {
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
		cmd.CommandID = uuid.New()
	}
	parent := uuid.Nil
	if cmd.ParentID != nil {
		parent = *cmd.ParentID
	}
	node, err := model.NewFile(cmd.NamespaceID, cmd.ActorID, cmd.Name, parent)
	if err != nil {
		return nil, err
	}
	if cmd.ParentID == nil {
		node.ParentID = nil
	}
	node.ID = uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-node:"+cmd.CommandID.String()))
	objectKey := fmt.Sprintf("sessions/%s/%s", cmd.CommandID, uuid.NewString())
	info, err := h.store.Put(ctx, storage.PutRequest{ObjectKey: objectKey, Body: cmd.Body, Size: cmd.Size, SHA256: cmd.SHA256})
	if err != nil {
		return nil, err
	}
	published := true
	defer func() {
		if published {
			_ = h.store.Delete(context.Background(), info.ObjectKey)
		}
	}()
	blob := &model.Blob{ID: uuid.NewSHA1(uuid.NameSpaceURL, []byte(cmd.CommandID.String())), DedupDomain: cmd.OrganizationID, SHA256: info.SHA256, Size: info.Size, RefCount: 1, State: model.BlobStateAvailable}
	revisionID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-revision:"+cmd.CommandID.String()))
	revision, err := model.NewRevisionWithID(revisionID, node.ID, 1, blob.ID, info.Size, cmd.MIMEType, cmd.ActorID)
	if err != nil {
		return nil, err
	}
	providerID := cmd.ReplicaProviderID
	if providerID == uuid.Nil {
		return nil, errors.New("replica provider id is required")
	}
	replica := &model.StorageReplica{ID: uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-replica:"+cmd.CommandID.String())), BlobID: blob.ID, ProviderID: providerID, ObjectKey: info.ObjectKey, Checksum: info.SHA256, State: model.ReplicaStateAvailable}
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.CreateNode(txCtx, node); err != nil {
			return err
		}
		if err := h.repo.SaveRevision(txCtx, revision, blob, replica); err != nil {
			return err
		}
		if err := h.repo.SetCurrentRevision(txCtx, node.ID, revision.ID()); err != nil {
			return err
		}
		if err := h.repo.ApplyQuotaUsage(txCtx, cmd.WorkspaceID, info.Size, 1); err != nil {
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
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.file.put_local", true, nil)
	})
	if err != nil {
		return nil, err
	}
	published = false
	return node, nil
}

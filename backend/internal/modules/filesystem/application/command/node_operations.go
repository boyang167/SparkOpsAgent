package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type RenameNode struct {
	CommandID      uuid.UUID
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	NodeID         uuid.UUID
	Name           string
	Authorization
}

type RenameNodeHandler struct {
	repo  repository.FilesystemRepository
	tx    postgres.Transactor
	audit auditPort
	bus   messaging.Bus
}

func NewRenameNodeHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *RenameNodeHandler {
	requireMutationDependencies(repo, tx)
	return &RenameNodeHandler{repo: repo, tx: tx, bus: bus, audit: logger}
}

func (h *RenameNodeHandler) Handle(ctx context.Context, cmd RenameNode) (err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ctx = withResourceID(ctx, cmd.NodeID.String())
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.rename")
	defer finishAudit(&err)
	if authErr := requireAllowed(cmd.Authorization, cmd.WorkspaceID, cmd.NodeID, model.ResourcePermissionWrite); authErr != nil {
		return deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.rename", authErr)
	}
	err = withinIdempotentMutation(ctx, h.tx, func(txCtx context.Context) error {
		node, err := h.repo.GetNode(txCtx, cmd.NamespaceID, cmd.NodeID)
		if err != nil {
			return err
		}
		txCtx = withResourceID(txCtx, node.ID.String())
		ns, err := h.repo.GetNamespaceByWorkspace(txCtx, cmd.WorkspaceID)
		if err != nil {
			return err
		}
		txCtx = withResourceID(txCtx, node.ID.String())
		if ns.ID != cmd.NamespaceID || ns.OrganizationID != cmd.OrganizationID {
			return domain.ErrNamespaceMismatch
		}
		if ns.RootNodeID != nil && *ns.RootNodeID == node.ID {
			return domain.ErrRootMutation
		}
		requestHash := mutationRequestHash("rename", cmd.WorkspaceID, cmd.NamespaceID, cmd.NodeID, cmd.Name, cmd.ActorID)
		completed, _, err := claimMutation(txCtx, h.repo, cmd.CommandID, cmd.WorkspaceID, "rename", requestHash, &cmd.NodeID, nil)
		if err != nil || completed {
			return err
		}
		if err := h.repo.RenameNode(txCtx, cmd.NodeID, cmd.Name); err != nil {
			return err
		}
		if h.bus != nil {
			if err := h.bus.Publish(txCtx, event.NodeRenamed{BaseEvent: events.NewBaseEvent(event.NodeRenamedName), NodeID: cmd.NodeID}); err != nil {
				return err
			}
		}
		if err := completeMutation(txCtx, h.repo, cmd.CommandID, &cmd.NodeID, nil); err != nil {
			return err
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.rename", true, nil)
	})
	if err != nil {
		return errors.Join(err, logMutation(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.rename", false, err))
	}
	return nil
}

type MoveNode struct {
	CommandID      uuid.UUID
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	NodeID         uuid.UUID
	NewParentID    *uuid.UUID
	NewName        string
	Authorization
}

type MoveNodeHandler struct {
	repo         repository.FilesystemRepository
	tx           postgres.Transactor
	audit        auditPort
	bus          messaging.Bus
	retryBackoff func(int) time.Duration
	retryWait    func(context.Context, time.Duration) error
}

func NewMoveNodeHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *MoveNodeHandler {
	requireMutationDependencies(repo, tx)
	return &MoveNodeHandler{
		repo: repo, tx: tx, bus: bus, audit: logger,
		retryBackoff: mutationRetryBackoff,
		retryWait:    waitForMutationRetry,
	}
}

func (h *MoveNodeHandler) Handle(ctx context.Context, cmd MoveNode) (err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ctx = withResourceID(ctx, cmd.NodeID.String())
	targetParentID := ""
	if cmd.NewParentID != nil {
		targetParentID = cmd.NewParentID.String()
	}
	ctx = withAuditDetail(ctx, map[string]any{"source_node_id": cmd.NodeID.String(), "target_parent_id": targetParentID})
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.move")
	defer finishAudit(&err)
	err = withinIdempotentMutationWithRetry(ctx, h.tx, func(txCtx context.Context) error {
		node, err := h.repo.GetNode(txCtx, cmd.NamespaceID, cmd.NodeID)
		if err != nil {
			return err
		}
		txCtx = withResourceID(txCtx, node.ID.String())
		ns, err := h.repo.GetNamespaceByWorkspace(txCtx, cmd.WorkspaceID)
		if err != nil {
			return err
		}
		if ns.ID != cmd.NamespaceID || ns.OrganizationID != cmd.OrganizationID {
			return domain.ErrNamespaceMismatch
		}
		if ns.RootNodeID != nil && *ns.RootNodeID == node.ID {
			return domain.ErrRootMutation
		}
		name := cmd.NewName
		if name == "" {
			name = node.Name
		}
		targetParent := cmd.NewParentID
		if targetParent == nil {
			targetParent = ns.RootNodeID
		}
		if targetParent == nil {
			return domain.ErrParentNotFound
		}
		if authErr := requireAllowed(cmd.Authorization, cmd.WorkspaceID, cmd.NodeID, model.ResourcePermissionWrite); authErr != nil {
			return authErr
		}
		if authErr := requireAllowed(cmd.Authorization, cmd.WorkspaceID, *targetParent, model.ResourcePermissionWrite); authErr != nil {
			return authErr
		}
		requestHash := mutationRequestHash("move", cmd.WorkspaceID, cmd.NamespaceID, cmd.NodeID, targetParent, name, cmd.ActorID)
		completed, _, err := claimMutation(txCtx, h.repo, cmd.CommandID, cmd.WorkspaceID, "move", requestHash, &cmd.NodeID, nil)
		if err != nil || completed {
			return err
		}
		if err := h.repo.MoveNodeSafely(txCtx, cmd.NamespaceID, node.ID, targetParent, name); err != nil {
			return err
		}
		if h.bus != nil {
			if err := h.bus.Publish(txCtx, event.NodeMoved{BaseEvent: events.NewBaseEvent(event.NodeMovedName), NodeID: node.ID, ParentID: *targetParent}); err != nil {
				return err
			}
		}
		if err := completeMutation(txCtx, h.repo, cmd.CommandID, &cmd.NodeID, nil); err != nil {
			return err
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.move", true, nil)
	}, h.retryBackoff, h.retryWait)
	if err != nil {
		return errors.Join(err, logMutation(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.move", false, err))
	}
	return nil
}

type TrashNode struct {
	CommandID      uuid.UUID
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	NodeID         uuid.UUID
	Authorization
}

type TrashNodeHandler struct {
	repo  repository.FilesystemRepository
	tx    postgres.Transactor
	audit auditPort
	bus   messaging.Bus
}

func NewTrashNodeHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *TrashNodeHandler {
	requireMutationDependencies(repo, tx)
	return &TrashNodeHandler{repo: repo, tx: tx, bus: bus, audit: logger}
}

func (h *TrashNodeHandler) Handle(ctx context.Context, cmd TrashNode) (err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ctx = withResourceID(ctx, cmd.NodeID.String())
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.trash")
	defer finishAudit(&err)
	if authErr := requireAllowed(cmd.Authorization, cmd.WorkspaceID, cmd.NodeID, model.ResourcePermissionDelete); authErr != nil {
		return deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.trash", authErr)
	}
	err = withinIdempotentMutation(ctx, h.tx, func(txCtx context.Context) error {
		ns, err := h.repo.GetNamespaceByWorkspace(txCtx, cmd.WorkspaceID)
		if err != nil {
			return err
		}
		if ns.ID != cmd.NamespaceID || ns.OrganizationID != cmd.OrganizationID {
			return domain.ErrNamespaceMismatch
		}
		requestHash := mutationRequestHash("trash", cmd.WorkspaceID, cmd.NamespaceID, cmd.NodeID, cmd.ActorID)
		completed, _, err := claimMutation(txCtx, h.repo, cmd.CommandID, cmd.WorkspaceID, "trash", requestHash, &cmd.NodeID, nil)
		if err != nil || completed {
			return err
		}
		node, err := h.repo.GetNodeIncludingTrashed(txCtx, cmd.NamespaceID, cmd.NodeID)
		if err != nil {
			return err
		}
		if ns.RootNodeID != nil && *ns.RootNodeID == node.ID {
			return domain.ErrRootMutation
		}
		if node.State == model.NodeStateTrashed || node.State == model.NodeStatePurged {
			return domain.ErrNodeStateConflict
		}
		if err := h.repo.TrashNode(txCtx, node.ID); err != nil {
			return err
		}
		if h.bus != nil {
			if err := h.bus.Publish(txCtx, event.NodeTrashed{BaseEvent: events.NewBaseEvent(event.NodeTrashedName), NodeID: node.ID}); err != nil {
				return err
			}
		}
		if err := completeMutation(txCtx, h.repo, cmd.CommandID, &cmd.NodeID, nil); err != nil {
			return err
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.trash", true, nil)
	})
	if err != nil {
		return errors.Join(err, logMutation(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.trash", false, err))
	}
	return nil
}

type RestoreRevision struct {
	CommandID      uuid.UUID
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	NodeID         uuid.UUID
	RevisionID     uuid.UUID
	Authorization
}

type RestoreRevisionHandler struct {
	repo  repository.FilesystemRepository
	tx    postgres.Transactor
	audit auditPort
	bus   messaging.Bus
}

func NewRestoreRevisionHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *RestoreRevisionHandler {
	requireMutationDependencies(repo, tx)
	return &RestoreRevisionHandler{repo: repo, tx: tx, bus: bus, audit: logger}
}

func (h *RestoreRevisionHandler) Handle(ctx context.Context, cmd RestoreRevision) (err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ctx = withResourceID(ctx, cmd.NodeID.String())
	ctx = withAuditDetail(ctx, map[string]any{"node_id": cmd.NodeID.String(), "revision_id": cmd.RevisionID.String()})
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.revision.restore")
	defer finishAudit(&err)
	if authErr := requireAllowed(cmd.Authorization, cmd.WorkspaceID, cmd.NodeID, model.ResourcePermissionWrite); authErr != nil {
		return deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.revision.restore", authErr)
	}
	err = withinIdempotentMutation(ctx, h.tx, func(txCtx context.Context) error {
		ns, err := h.repo.GetNamespaceByWorkspace(txCtx, cmd.WorkspaceID)
		if err != nil {
			return err
		}
		if ns.ID != cmd.NamespaceID || ns.OrganizationID != cmd.OrganizationID {
			return domain.ErrNamespaceMismatch
		}
		requestHash := mutationRequestHash("restore", cmd.WorkspaceID, cmd.NamespaceID, cmd.NodeID, cmd.RevisionID, cmd.ActorID)
		completed, _, err := claimMutation(txCtx, h.repo, cmd.CommandID, cmd.WorkspaceID, "restore", requestHash, &cmd.NodeID, &cmd.RevisionID)
		if err != nil || completed {
			return err
		}
		if _, err := h.repo.GetNode(txCtx, cmd.NamespaceID, cmd.NodeID); err != nil {
			return err
		}
		if _, err := h.repo.GetRevision(txCtx, cmd.NodeID, cmd.RevisionID); err != nil {
			return err
		}
		if err := h.repo.SetCurrentRevision(txCtx, cmd.NodeID, cmd.RevisionID); err != nil {
			return err
		}
		if h.bus != nil {
			if err := h.bus.Publish(txCtx, event.RevisionRestored{BaseEvent: events.NewBaseEvent(event.RevisionRestoredName), NodeID: cmd.NodeID, RevisionID: cmd.RevisionID}); err != nil {
				return err
			}
		}
		if err := completeMutation(txCtx, h.repo, cmd.CommandID, &cmd.NodeID, &cmd.RevisionID); err != nil {
			return err
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.revision.restore", true, nil)
	})
	if err != nil {
		return errors.Join(err, logMutation(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.revision.restore", false, err))
	}
	return nil
}

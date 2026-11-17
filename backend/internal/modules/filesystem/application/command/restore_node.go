package command

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type RestoreNode struct {
	CommandID      uuid.UUID
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	NodeID         uuid.UUID
	Authorization
}

type RestoreNodeHandler struct {
	repo  repository.FilesystemRepository
	tx    postgres.Transactor
	audit auditPort
	bus   messaging.Bus
}

func NewRestoreNodeHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *RestoreNodeHandler {
	requireMutationDependencies(repo, tx)
	return &RestoreNodeHandler{repo: repo, tx: tx, bus: bus, audit: logger}
}

func (h *RestoreNodeHandler) Handle(ctx context.Context, cmd RestoreNode) (err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ctx = withResourceID(ctx, cmd.NodeID.String())
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.restore")
	defer finishAudit(&err)
	if !cmd.Allowed {
		err := authorizationDenied(cmd.Reason)
		return deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.restore", err)
	}
	err = withinIdempotentMutation(ctx, h.tx, func(txCtx context.Context) error {
		ns, err := h.repo.GetNamespace(txCtx, cmd.NamespaceID)
		if err != nil {
			return err
		}
		if ns.OrganizationID != cmd.OrganizationID ||
			(ns.NamespaceType == model.NamespaceTypeWorkspace && ns.WorkspaceID != cmd.WorkspaceID) {
			return domain.ErrNamespaceMismatch
		}
		node, err := h.repo.GetNodeIncludingTrashed(txCtx, cmd.NamespaceID, cmd.NodeID)
		if err != nil {
			return err
		}
		parentID := node.ParentID
		if parentID == nil {
			parentID = ns.RootNodeID
		}
		if parentID == nil {
			return domain.ErrRestoreParentUnavailable
		}
		if err := requireAllowed(cmd.Authorization, cmd.WorkspaceID, *parentID, model.ResourcePermissionWrite); err != nil {
			return err
		}
		requestHash := mutationRequestHash("restore_node", cmd.WorkspaceID, cmd.NamespaceID, cmd.NodeID, cmd.ActorID)
		completed, _, err := claimMutation(txCtx, h.repo, cmd.CommandID, cmd.WorkspaceID, "restore_node", requestHash, &cmd.NodeID, nil)
		if err != nil || completed {
			return err
		}
		if node.State != model.NodeStateTrashed {
			return domain.ErrNodeStateConflict
		}
		if err := h.repo.RestoreNode(txCtx, cmd.NamespaceID, cmd.NodeID); err != nil {
			return err
		}
		if h.bus != nil {
			if err := h.bus.Publish(txCtx, event.NodeRestored{BaseEvent: events.NewBaseEvent(event.NodeRestoredName), NodeID: node.ID}); err != nil {
				return err
			}
		}
		if err := completeMutation(txCtx, h.repo, cmd.CommandID, &cmd.NodeID, nil); err != nil {
			return err
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.restore", true, nil)
	})
	if err != nil {
		return errors.Join(err, logMutation(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.restore", false, err))
	}
	return nil
}

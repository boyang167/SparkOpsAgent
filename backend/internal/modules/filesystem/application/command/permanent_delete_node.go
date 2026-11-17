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

type PermanentDeleteNode struct {
	CommandID      uuid.UUID
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	NodeID         uuid.UUID
	Authorization
}

type PermanentDeleteNodeHandler struct {
	repo  repository.FilesystemRepository
	tx    postgres.Transactor
	audit auditPort
	bus   messaging.Bus
}

func NewPermanentDeleteNodeHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *PermanentDeleteNodeHandler {
	requireMutationDependencies(repo, tx)
	return &PermanentDeleteNodeHandler{repo: repo, tx: tx, bus: bus, audit: logger}
}

func (h *PermanentDeleteNodeHandler) Handle(ctx context.Context, cmd PermanentDeleteNode) (err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ctx = withResourceID(ctx, cmd.NodeID.String())
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.permanent_delete")
	defer finishAudit(&err)
	if !cmd.Allowed {
		err := authorizationDenied(cmd.Reason)
		return deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.permanent_delete", err)
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
		if err := requireAllowed(cmd.Authorization, cmd.WorkspaceID, cmd.NodeID, model.ResourcePermissionDelete); err != nil {
			return err
		}
		node, err := h.repo.GetNodeIncludingTrashed(txCtx, cmd.NamespaceID, cmd.NodeID)
		if err != nil {
			return err
		}
		if ns.RootNodeID != nil && *ns.RootNodeID == node.ID {
			return domain.ErrRootMutation
		}
		requestHash := mutationRequestHash("permanent_delete", cmd.WorkspaceID, cmd.NamespaceID, cmd.NodeID, cmd.ActorID)
		completed, _, err := claimMutation(txCtx, h.repo, cmd.CommandID, cmd.WorkspaceID, "permanent_delete", requestHash, &cmd.NodeID, nil)
		if err != nil || completed {
			return err
		}
		if node.State != model.NodeStateTrashed {
			return domain.ErrNodeStateConflict
		}
		now := time.Now().UTC()
		if _, err := h.repo.PurgeTrashedSubtree(txCtx, cmd.NamespaceID, cmd.NodeID, now); err != nil {
			return err
		}
		if h.bus != nil {
			if err := h.bus.Publish(txCtx, event.NodePurged{BaseEvent: events.NewBaseEvent(event.NodePurgedName), NodeID: node.ID}); err != nil {
				return err
			}
		}
		if err := completeMutation(txCtx, h.repo, cmd.CommandID, &cmd.NodeID, nil); err != nil {
			return err
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.permanent_delete", true, nil)
	})
	if err != nil {
		return errors.Join(err, logMutation(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.permanent_delete", false, err))
	}
	return nil
}

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

type CopyNode struct {
	CommandID      uuid.UUID
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	SourceNodeID   uuid.UUID
	ParentID       *uuid.UUID
	Name           string
	Authorization
}

type CopyNodeHandler struct {
	repo         repository.FilesystemRepository
	tx           postgres.Transactor
	audit        auditPort
	bus          messaging.Bus
	retryBackoff func(int) time.Duration
	retryWait    func(context.Context, time.Duration) error
}

func NewCopyNodeHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *CopyNodeHandler {
	requireMutationDependencies(repo, tx)
	return &CopyNodeHandler{
		repo: repo, tx: tx, bus: bus, audit: logger,
		retryBackoff: mutationRetryBackoff,
		retryWait:    waitForMutationRetry,
	}
}

func (h *CopyNodeHandler) Handle(ctx context.Context, cmd CopyNode) (out *model.Node, err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ctx = withResourceID(ctx, cmd.SourceNodeID.String())
	targetParentID := ""
	if cmd.ParentID != nil {
		targetParentID = cmd.ParentID.String()
	}
	ctx = withAuditDetail(ctx, map[string]any{"source_node_id": cmd.SourceNodeID.String(), "target_parent_id": targetParentID})
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.copy")
	defer finishAudit(&err)
	if !cmd.Allowed {
		err := authorizationDenied(cmd.Reason)
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.copy", err)
	}
	var result *model.Node
	var requestHash string
	err = withinIdempotentMutationWithRetry(ctx, h.tx, func(txCtx context.Context) error {
		ns, err := h.repo.GetNamespaceByWorkspace(txCtx, cmd.WorkspaceID)
		if err != nil {
			return err
		}
		if ns.ID != cmd.NamespaceID || ns.OrganizationID != cmd.OrganizationID {
			return domain.ErrNamespaceMismatch
		}
		source, err := h.repo.GetNode(txCtx, cmd.NamespaceID, cmd.SourceNodeID)
		if err != nil {
			return err
		}
		if source.Type != model.NodeTypeFile || source.CurrentRevisionID == nil {
			return domain.ErrUnsupportedNodeType
		}
		parent := uuid.Nil
		if cmd.ParentID != nil {
			parent = *cmd.ParentID
		} else if ns.RootNodeID != nil {
			parent = *ns.RootNodeID
		}
		name := cmd.Name
		if name == "" {
			name = source.Name
		}
		result, err = model.NewFile(cmd.NamespaceID, cmd.ActorID, name, parent)
		if err != nil {
			return err
		}
		if cmd.CommandID != uuid.Nil {
			result.ID = uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-copy:"+cmd.CommandID.String()))
		}
		revisionID := uuid.New()
		if cmd.CommandID != uuid.Nil {
			revisionID = uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-copy-revision:"+cmd.CommandID.String()))
		}
		requestHash = mutationRequestHash("copy", cmd.WorkspaceID, cmd.NamespaceID, source.ID, *source.CurrentRevisionID, parent, result.Name, cmd.ActorID)
		completed, claimed, err := claimMutation(txCtx, h.repo, cmd.CommandID, cmd.WorkspaceID, "copy", requestHash, &result.ID, &revisionID)
		if err != nil {
			return err
		}
		if completed {
			if claimed.NodeID == nil || claimed.RevisionID == nil {
				return domain.ErrOutcomeUnknown
			}
			result, err = h.repo.GetNode(txCtx, cmd.NamespaceID, *claimed.NodeID)
			if err != nil {
				return err
			}
			_, err = h.repo.GetRevision(txCtx, result.ID, *claimed.RevisionID)
			return err
		}
		txCtx = withResourceID(txCtx, result.ID.String())
		sourceRevision, err := h.repo.GetRevision(txCtx, source.ID, *source.CurrentRevisionID)
		if err != nil {
			return err
		}
		blob, err := h.repo.GetBlob(txCtx, sourceRevision.BlobID())
		if err != nil {
			return err
		}
		replica, err := h.repo.GetAvailableReplica(txCtx, blob.ID, uuid.Nil)
		if err != nil {
			return err
		}
		if err := h.repo.CreateNode(txCtx, result); err != nil {
			return err
		}
		newRevision, err := model.NewRevisionWithID(revisionID, result.ID, 1, blob.ID, sourceRevision.Size(), sourceRevision.MIMEType(), cmd.ActorID)
		if err != nil {
			return err
		}
		if err := h.repo.SaveRevision(txCtx, newRevision, blob, replica); err != nil {
			return err
		}
		if err := h.repo.SetCurrentRevision(txCtx, result.ID, newRevision.ID()); err != nil {
			return err
		}
		if err := h.repo.IncrementBlobRef(txCtx, blob.ID); err != nil {
			return err
		}
		if err := h.repo.ApplyQuotaUsage(txCtx, cmd.WorkspaceID, sourceRevision.Size(), 1); err != nil {
			return err
		}
		if h.bus != nil {
			if err := h.bus.Publish(txCtx, event.NodeCopied{
				BaseEvent: events.NewBaseEvent(event.NodeCopiedName), SourceNodeID: source.ID,
				NodeID: result.ID, RevisionID: revisionID,
			}); err != nil {
				return err
			}
		}
		if err := completeMutation(txCtx, h.repo, cmd.CommandID, &result.ID, &revisionID); err != nil {
			return err
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.copy", true, nil)
	}, h.retryBackoff, h.retryWait)
	if err != nil {
		if cmd.CommandID != uuid.Nil && errors.Is(err, domain.ErrNodeIDConflict) {
			for attempt := 0; attempt < 5; attempt++ {
				claimed, readErr := h.repo.GetCommand(ctx, cmd.CommandID)
				if readErr == nil {
					if claimed.WorkspaceID != cmd.WorkspaceID || claimed.Operation != "copy" || claimed.RequestHash != requestHash {
						return nil, domain.ErrIdempotencyConflict
					}
					if claimed.State == "COMPLETED" && claimed.NodeID != nil && claimed.RevisionID != nil {
						authoritative, nodeErr := h.repo.GetNode(ctx, cmd.NamespaceID, *claimed.NodeID)
						if nodeErr == nil {
							if _, revisionErr := h.repo.GetRevision(ctx, authoritative.ID, *claimed.RevisionID); revisionErr == nil {
								markMutationReplay(ctx)
								return authoritative, nil
							}
						}
					}
				}
				if waitErr := h.retryWait(ctx, h.retryBackoff(attempt)); waitErr != nil {
					err = waitErr
					break
				}
			}
		}
		return nil, errors.Join(err, logMutation(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.copy", false, err))
	}
	return result, err
}

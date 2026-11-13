package command

import (
	"context"
	"errors"
	"strings"
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

type CommitTransfer struct {
	SessionID   uuid.UUID
	WorkspaceID uuid.UUID
}

type CommitTransferResult struct {
	NodeID     uuid.UUID
	RevisionID uuid.UUID
}

type CommitTransferHandler struct {
	repo repository.FilesystemRepository
	tx   postgres.Transactor
	bus  messaging.Bus
}

func NewCommitTransferHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus) *CommitTransferHandler {
	requireMutationDependencies(repo, tx)
	return &CommitTransferHandler{repo: repo, tx: tx, bus: bus}
}

func transferNodeID(sessionID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-transfer-node:"+sessionID.String()))
}

func transferRevisionID(sessionID uuid.UUID) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-transfer-revision:"+sessionID.String()))
}

func (h *CommitTransferHandler) Handle(ctx context.Context, cmd CommitTransfer) (result CommitTransferResult, err error) {
	if cmd.SessionID == uuid.Nil {
		return result, model.ErrInvalidTransfer
	}
	nodeID := transferNodeID(cmd.SessionID)
	revisionID := transferRevisionID(cmd.SessionID)
	preview, previewErr := h.repo.GetTransferByID(ctx, cmd.SessionID)
	if previewErr == nil && preview.Status() == model.TransferCompleted {
		if cmd.WorkspaceID != uuid.Nil && preview.WorkspaceID() != cmd.WorkspaceID {
			return result, domain.ErrNamespaceMismatch
		}
		return CommitTransferResult{NodeID: nodeID, RevisionID: revisionID}, nil
	}
	if previewErr != nil && !errors.Is(previewErr, domain.ErrTransferNotFound) {
		return result, previewErr
	}

	var completed CommitTransferResult
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		session, lockErr := h.repo.LockTransfer(txCtx, cmd.SessionID)
		if lockErr != nil {
			return lockErr
		}
		if cmd.WorkspaceID != uuid.Nil && session.WorkspaceID() != cmd.WorkspaceID {
			return domain.ErrNamespaceMismatch
		}
		if session.Status() == model.TransferCompleted {
			completed = CommitTransferResult{NodeID: nodeID, RevisionID: revisionID}
			return nil
		}
		if session.Status() != model.TransferScanning {
			return domain.ErrTransferStatusConflict
		}
		ns, nsErr := h.repo.GetNamespaceByWorkspace(txCtx, session.WorkspaceID())
		if nsErr != nil {
			return nsErr
		}
		now := time.Now().UTC()
		version := session.Version()
		if err := session.StartCommit(version, now); err != nil {
			return err
		}
		if err := h.repo.SaveTransferStateCAS(txCtx, session.WorkspaceID(), session.ID(), version, repository.TransferStatePatch{
			StatusFrom: model.TransferScanning, StatusTo: model.TransferCommitting,
		}); err != nil {
			return err
		}
		session, lockErr = h.repo.LockTransfer(txCtx, cmd.SessionID)
		if lockErr != nil {
			return lockErr
		}
		size, ok := session.ActualSize()
		if !ok {
			return model.ErrTransferIntegrity
		}
		checksum := strings.ToLower(session.ActualSHA256())
		providerID := session.ProviderID()
		if providerID == nil {
			return model.ErrInvalidTransfer
		}
		parentID := session.TargetParentID()
		if parentID == nil {
			parentID = ns.RootNodeID
		}
		if parentID == nil {
			return domain.ErrParentNotFound
		}
		fileNode, nodeErr := model.NewFile(session.NamespaceID(), session.CreatedBy(), session.TargetName(), *parentID)
		if nodeErr != nil {
			return nodeErr
		}
		fileNode.ID = nodeID
		_, existingErr := h.repo.GetNodeIncludingTrashed(txCtx, session.NamespaceID(), nodeID)
		if errors.Is(existingErr, domain.ErrNodeNotFound) {
			if err := h.repo.CreateNode(txCtx, fileNode); err != nil && !errors.Is(err, domain.ErrNodeIDConflict) {
				return err
			}
		} else if existingErr != nil {
			return existingErr
		}
		blob, blobErr := h.repo.AcquireBlob(txCtx, ns.OrganizationID, checksum, size)
		if blobErr != nil {
			return blobErr
		}
		verifiedAt := now
		replica := &model.StorageReplica{
			ID: uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-transfer-replica:"+session.ID().String())),
			BlobID: blob.ID, ProviderID: *providerID, ObjectKey: session.TemporaryObjectKey(), Checksum: checksum,
			State: model.ReplicaStateAvailable, VerifiedAt: &verifiedAt,
		}
		revision, revisionErr := model.NewRevisionWithID(revisionID, nodeID, 1, blob.ID, size, "application/octet-stream", session.CreatedBy())
		if revisionErr != nil {
			return revisionErr
		}
		if err := h.repo.SaveRevision(txCtx, revision, blob, replica); err != nil {
			return err
		}
		if err := h.repo.SetCurrentRevision(txCtx, nodeID, revisionID); err != nil {
			return err
		}
		if err := h.repo.CommitQuotaReservation(txCtx, session.WorkspaceID(), session.ID()); err != nil {
			return err
		}
		if h.bus != nil {
			createdNode := errors.Is(existingErr, domain.ErrNodeNotFound)
			events := commitTransferEvents(session, fileNode, revision, createdNode)
			if publishErr := h.bus.Publish(txCtx, events...); publishErr != nil {
				return publishErr
			}
		}
		version = session.Version()
		if err := session.Complete(version, now); err != nil {
			return err
		}
		if err := h.repo.SaveTransferStateCAS(txCtx, session.WorkspaceID(), session.ID(), version, repository.TransferStatePatch{
			StatusFrom: model.TransferCommitting, StatusTo: model.TransferCompleted,
		}); err != nil {
			return err
		}
		completed = CommitTransferResult{NodeID: nodeID, RevisionID: revisionID}
		return nil
	})
	if err != nil {
		return result, err
	}
	return completed, nil
}

func commitTransferEvents(session *model.TransferSession, node *model.Node, revision *model.FileRevision, created bool) []sharedevents.Event {
	out := []sharedevents.Event{
		event.TransferCompleted{
			BaseEvent: sharedevents.NewBaseEvent(event.TransferCompletedName),
			SessionID: session.ID(), WorkspaceID: session.WorkspaceID(), NamespaceID: session.NamespaceID(),
			NodeID: node.ID, RevisionID: revision.ID(),
		},
	}
	if created {
		out = append([]sharedevents.Event{
			event.FileCreated{BaseEvent: sharedevents.NewBaseEvent(event.FileCreatedName), FileID: node.ID, NamespaceID: node.NamespaceID},
			event.FileRevisionCreated{
				BaseEvent: sharedevents.NewBaseEvent(event.FileRevisionCreatedName),
				FileID: node.ID, RevisionID: revision.ID(), RevisionNo: revision.RevisionNo(),
			},
		}, out...)
	} else {
		out = append([]sharedevents.Event{
			event.FileRevisionCreated{
				BaseEvent: sharedevents.NewBaseEvent(event.FileRevisionCreatedName),
				FileID: node.ID, RevisionID: revision.ID(), RevisionNo: revision.RevisionNo(),
			},
		}, out...)
	}
	return out
}

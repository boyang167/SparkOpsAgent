package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type AbortTransfer struct {
	ActorID     uuid.UUID
	WorkspaceID uuid.UUID
	NamespaceID uuid.UUID
	SessionID   uuid.UUID
	Authorization
}

type AbortTransferHandler struct {
	repo repository.FilesystemRepository
	tx   postgres.Transactor
}

func NewAbortTransferHandler(repo repository.FilesystemRepository, tx postgres.Transactor) *AbortTransferHandler {
	requireMutationDependencies(repo, tx)
	return &AbortTransferHandler{repo: repo, tx: tx}
}

func (h *AbortTransferHandler) Handle(ctx context.Context, cmd AbortTransfer) error {
	if cmd.SessionID == uuid.Nil {
		return model.ErrInvalidTransfer
	}
	preview, err := h.repo.GetTransferByID(ctx, cmd.SessionID)
	if err != nil {
		return err
	}
	namespace, err := h.repo.GetNamespace(ctx, preview.NamespaceID())
	if err != nil {
		return err
	}
	if err := validateAbortScope(cmd, namespace, preview); err != nil {
		return err
	}
	parent := preview.TargetParentID()
	if parent == nil {
		return domain.ErrParentNotFound
	}
	if authErr := requireAllowed(cmd.Authorization, cmd.WorkspaceID, *parent, model.ResourcePermissionWrite); authErr != nil {
		return authErr
	}
	return h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		session, lockErr := h.repo.LockTransfer(txCtx, cmd.SessionID)
		if lockErr != nil {
			return lockErr
		}
		if session.NamespaceID() != preview.NamespaceID() {
			return domain.ErrTransferNotFound
		}
		if err := validateAbortScope(cmd, namespace, session); err != nil {
			return err
		}
		switch session.Status() {
		case model.TransferAborted:
			return nil
		case model.TransferCompleted, model.TransferRejected, model.TransferFailed, model.TransferExpired:
			return domain.ErrTransferStatusConflict
		}
		statusFrom := session.Status()
		version := session.Version()
		now := time.Now().UTC()
		if err := session.Abort(version, now); err != nil {
			if errors.Is(err, model.ErrInvalidTransferTransition) && session.Status() == model.TransferAborted {
				return nil
			}
			return err
		}
		if err := h.repo.SaveTransferStateCAS(txCtx, session.WorkspaceID(), session.ID(), version, repository.TransferStatePatch{
			StatusFrom: statusFrom, StatusTo: model.TransferAborted,
		}); err != nil {
			return err
		}
		return h.repo.ReleaseNamespaceQuotaReservation(txCtx, session.NamespaceID(), cmd.SessionID)
	})
}

func validateAbortScope(cmd AbortTransfer, namespace *model.Namespace, session *model.TransferSession) error {
	if namespace == nil || session == nil || namespace.ID != session.NamespaceID() {
		return domain.ErrTransferNotFound
	}
	if cmd.NamespaceID != uuid.Nil && cmd.NamespaceID != session.NamespaceID() {
		return domain.ErrTransferNotFound
	}
	if namespace.NamespaceType == model.NamespaceTypeWorkspace {
		if cmd.WorkspaceID == uuid.Nil ||
			cmd.WorkspaceID != namespace.WorkspaceID ||
			session.WorkspaceID() != namespace.WorkspaceID {
			return domain.ErrTransferNotFound
		}
		return nil
	}
	if cmd.NamespaceID != uuid.Nil && cmd.NamespaceID != session.NamespaceID() {
		return domain.ErrTransferNotFound
	}
	// User/organization namespaces: uploads are routed through a workspace but are not workspace-scoped.
	if cmd.WorkspaceID != uuid.Nil {
		if session.WorkspaceID() != uuid.Nil && session.WorkspaceID() != cmd.WorkspaceID {
			return domain.ErrTransferNotFound
		}
	}
	return nil
}

package command

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
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
}

func NewRenameNodeHandler(repo repository.FilesystemRepository, tx postgres.Transactor, _ messaging.Bus, logger auditPort) *RenameNodeHandler {
	requireMutationDependencies(repo, tx)
	return &RenameNodeHandler{repo: repo, tx: tx, audit: logger}
}

func (h *RenameNodeHandler) Handle(ctx context.Context, cmd RenameNode) (err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.rename")
	defer finishAudit(&err)
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.rename", err)
	}
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
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
			return fmt.Errorf("filesystem root cannot be renamed")
		}
		if err := h.repo.RenameNode(txCtx, cmd.NodeID, cmd.Name); err != nil {
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
	retryBackoff func(int) time.Duration
	retryWait    func(context.Context, time.Duration) error
}

func NewMoveNodeHandler(repo repository.FilesystemRepository, tx postgres.Transactor, _ messaging.Bus, logger auditPort) *MoveNodeHandler {
	requireMutationDependencies(repo, tx)
	return &MoveNodeHandler{
		repo: repo, tx: tx, audit: logger,
		retryBackoff: func(attempt int) time.Duration {
			base := time.Duration(attempt+1) * time.Millisecond
			return base + time.Duration(rand.Int64N(int64(time.Millisecond)))
		},
		retryWait: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (h *MoveNodeHandler) Handle(ctx context.Context, cmd MoveNode) (err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.move")
	defer finishAudit(&err)
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.move", err)
	}
	for attempt := 0; attempt < 5; attempt++ {
		err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
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
				return fmt.Errorf("filesystem root cannot be moved")
			}
			name := cmd.NewName
			if name == "" {
				name = node.Name
			}
			targetParent := cmd.NewParentID
			if targetParent == nil {
				targetParent = ns.RootNodeID
			}
			if err := h.repo.MoveNodeSafely(txCtx, cmd.NamespaceID, node.ID, targetParent, name); err != nil {
				return err
			}
			return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.move", true, nil)
		})
		if !errors.Is(err, domain.ErrTransactionRetry) || attempt == 4 {
			break
		}
		if waitErr := h.retryWait(ctx, h.retryBackoff(attempt)); waitErr != nil {
			err = waitErr
			break
		}
	}
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
}

func NewTrashNodeHandler(repo repository.FilesystemRepository, tx postgres.Transactor, _ messaging.Bus, logger auditPort) *TrashNodeHandler {
	requireMutationDependencies(repo, tx)
	return &TrashNodeHandler{repo: repo, tx: tx, audit: logger}
}

func (h *TrashNodeHandler) Handle(ctx context.Context, cmd TrashNode) (err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.trash")
	defer finishAudit(&err)
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.trash", err)
	}
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		node, err := h.repo.GetNode(txCtx, cmd.NamespaceID, cmd.NodeID)
		if err != nil {
			return err
		}
		ns, err := h.repo.GetNamespaceByWorkspace(txCtx, cmd.WorkspaceID)
		if err != nil {
			return err
		}
		if ns.ID != cmd.NamespaceID || ns.OrganizationID != cmd.OrganizationID {
			return domain.ErrNamespaceMismatch
		}
		if ns.RootNodeID != nil && *ns.RootNodeID == node.ID {
			return fmt.Errorf("filesystem root cannot be trashed")
		}
		if node.State != model.NodeStateTrashed {
			if err := h.repo.TrashNode(txCtx, node.ID); err != nil {
				return err
			}
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
}

func NewRestoreRevisionHandler(repo repository.FilesystemRepository, tx postgres.Transactor, _ messaging.Bus, logger auditPort) *RestoreRevisionHandler {
	requireMutationDependencies(repo, tx)
	return &RestoreRevisionHandler{repo: repo, tx: tx, audit: logger}
}

func (h *RestoreRevisionHandler) Handle(ctx context.Context, cmd RestoreRevision) (err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.revision.restore")
	defer finishAudit(&err)
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.revision.restore", err)
	}
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		ns, err := h.repo.GetNamespaceByWorkspace(txCtx, cmd.WorkspaceID)
		if err != nil {
			return err
		}
		if ns.ID != cmd.NamespaceID || ns.OrganizationID != cmd.OrganizationID {
			return domain.ErrNamespaceMismatch
		}
		if _, err := h.repo.GetNode(txCtx, cmd.NamespaceID, cmd.NodeID); err != nil {
			return err
		}
		txCtx = withResourceID(txCtx, cmd.RevisionID.String())
		if _, err := h.repo.GetRevision(txCtx, cmd.NodeID, cmd.RevisionID); err != nil {
			return err
		}
		if err := h.repo.SetCurrentRevision(txCtx, cmd.NodeID, cmd.RevisionID); err != nil {
			return err
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.revision.restore", true, nil)
	})
	if err != nil {
		return errors.Join(err, logMutation(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.revision.restore", false, err))
	}
	return nil
}

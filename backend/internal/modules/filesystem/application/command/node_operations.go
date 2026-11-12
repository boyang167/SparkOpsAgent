package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type RenameNode struct {
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
	return &RenameNodeHandler{repo: repo, tx: tx, audit: logger}
}

func (h *RenameNodeHandler) Handle(ctx context.Context, cmd RenameNode) error {
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.rename", err)
	}
	return h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		node, err := h.repo.GetNode(txCtx, cmd.NamespaceID, cmd.NodeID)
		if err != nil {
			return err
		}
		if node.ParentID == nil {
			return fmt.Errorf("filesystem root cannot be renamed")
		}
		if err := h.repo.RenameNode(txCtx, cmd.NodeID, cmd.Name); err != nil {
			return err
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.rename", true, nil)
	})
}

type MoveNode struct {
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
	repo  repository.FilesystemRepository
	tx    postgres.Transactor
	audit auditPort
}

func NewMoveNodeHandler(repo repository.FilesystemRepository, tx postgres.Transactor, _ messaging.Bus, logger auditPort) *MoveNodeHandler {
	return &MoveNodeHandler{repo: repo, tx: tx, audit: logger}
}

func (h *MoveNodeHandler) Handle(ctx context.Context, cmd MoveNode) error {
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.move", err)
	}
	return h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		node, err := h.repo.GetNode(txCtx, cmd.NamespaceID, cmd.NodeID)
		if err != nil {
			return err
		}
		ns, err := h.repo.GetNamespaceByWorkspace(txCtx, cmd.WorkspaceID)
		if err != nil {
			return err
		}
		if ns.RootNodeID != nil && *ns.RootNodeID == node.ID {
			return fmt.Errorf("filesystem root cannot be moved")
		}
		if cmd.NewParentID != nil {
			if *cmd.NewParentID == node.ID {
				return domain.ErrDirectoryCycle
			}
			visited := map[uuid.UUID]bool{node.ID: true}
			current := *cmd.NewParentID
			for depth := 0; depth < 10000; depth++ {
				if visited[current] {
					return domain.ErrDirectoryCycle
				}
				visited[current] = true
				parent, getErr := h.repo.GetNode(txCtx, cmd.NamespaceID, current)
				if getErr != nil {
					return getErr
				}
				if parent.ParentID == nil {
					break
				}
				current = *parent.ParentID
			}
			if len(visited) >= 10000 {
				return domain.ErrDirectoryCycle
			}
		}
		name := cmd.NewName
		if name == "" {
			name = node.Name
		}
		if err := h.repo.MoveNode(txCtx, node.ID, cmd.NewParentID, name); err != nil {
			return err
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.move", true, nil)
	})
}

type TrashNode struct {
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
	return &TrashNodeHandler{repo: repo, tx: tx, audit: logger}
}

func (h *TrashNodeHandler) Handle(ctx context.Context, cmd TrashNode) error {
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.trash", err)
	}
	return h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		node, err := h.repo.GetNode(txCtx, cmd.NamespaceID, cmd.NodeID)
		if err != nil {
			return err
		}
		ns, err := h.repo.GetNamespaceByWorkspace(txCtx, cmd.WorkspaceID)
		if err != nil {
			return err
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
}

type RestoreRevision struct {
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
	return &RestoreRevisionHandler{repo: repo, tx: tx, audit: logger}
}

func (h *RestoreRevisionHandler) Handle(ctx context.Context, cmd RestoreRevision) error {
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.revision.restore", err)
	}
	return h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := h.repo.GetRevision(txCtx, cmd.NodeID, cmd.RevisionID); err != nil {
			return err
		}
		if err := h.repo.SetCurrentRevision(txCtx, cmd.NodeID, cmd.RevisionID); err != nil {
			return err
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.revision.restore", true, nil)
	})
}

package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type CopyNode struct {
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
	repo  repository.FilesystemRepository
	tx    postgres.Transactor
	audit auditPort
}

func NewCopyNodeHandler(repo repository.FilesystemRepository, tx postgres.Transactor, logger auditPort) *CopyNodeHandler {
	return &CopyNodeHandler{repo: repo, tx: tx, audit: logger}
}

func (h *CopyNodeHandler) Handle(ctx context.Context, cmd CopyNode) (*model.Node, error) {
	if !cmd.Allowed {
		err := errors.New(cmd.Reason)
		if err.Error() == "" {
			err = errors.New("authorization denied")
		}
		return nil, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.copy", err)
	}
	var result *model.Node
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		source, err := h.repo.GetNode(txCtx, cmd.NamespaceID, cmd.SourceNodeID)
		if err != nil {
			return err
		}
		if source.Type != model.NodeTypeFile || source.CurrentRevisionID == nil {
			return fmt.Errorf("filesystem only file copy is supported")
		}
		parent := uuid.Nil
		if cmd.ParentID != nil {
			parent = *cmd.ParentID
		}
		name := cmd.Name
		if name == "" {
			name = source.Name
		}
		result, err = model.NewFile(cmd.NamespaceID, cmd.ActorID, name, parent)
		if err != nil {
			return err
		}
		if cmd.ParentID == nil {
			result.ParentID = nil
		}
		revision, err := h.repo.GetRevision(txCtx, source.ID, *source.CurrentRevisionID)
		if err != nil {
			return err
		}
		blob, err := h.repo.GetBlob(txCtx, revision.BlobID())
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
		newRevision, err := model.NewRevision(result.ID, revision.RevisionNo(), blob.ID, revision.Size(), revision.MIMEType(), cmd.ActorID)
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
		if err := h.repo.ApplyQuotaUsage(txCtx, cmd.WorkspaceID, revision.Size(), 1); err != nil {
			return err
		}
		return logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.node.copy", true, nil)
	})
	return result, err
}

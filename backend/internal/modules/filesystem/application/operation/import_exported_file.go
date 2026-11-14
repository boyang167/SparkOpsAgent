package operation

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type ImportExportedFile struct {
	ExportID             uuid.UUID
	TargetOrganizationID uuid.UUID
	TargetNamespaceID    uuid.UUID
	TargetParentID       uuid.UUID
	OwnerID              uuid.UUID
	Name                 string
}

type ImportResult struct {
	NodeID       uuid.UUID
	SourceNodeID uuid.UUID
}

type exportedNodeCreator interface {
	CreateNode(context.Context, *model.Node) error
}

type ImportExportedFileHandler struct {
	nodes exportedNodeCreator
	store repository.CapabilityRepository
}

func NewImportExportedFileHandler(nodes exportedNodeCreator, store repository.CapabilityRepository) *ImportExportedFileHandler {
	if nodes == nil || store == nil {
		panic("import exported file requires node creator and store")
	}
	return &ImportExportedFileHandler{nodes: nodes, store: store}
}

func (h *ImportExportedFileHandler) Handle(ctx context.Context, cmd ImportExportedFile) (ImportResult, error) {
	task, err := h.store.GetExportTask(ctx, cmd.ExportID)
	if err != nil {
		return ImportResult{}, err
	}
	classification := task.Classification
	switch classification {
	case model.TransferClassificationPublic, model.TransferClassificationInternal,
		model.TransferClassificationConfidential, model.TransferClassificationRestricted:
	default:
		classification = model.TransferClassificationRestricted
	}
	if cmd.TargetOrganizationID != uuid.Nil && cmd.TargetOrganizationID != task.TargetOrganizationID {
		return ImportResult{}, domain.ErrControlledExportScope
	}
	if task.TargetNodeID != nil {
		return ImportResult{NodeID: *task.TargetNodeID, SourceNodeID: task.SourceNodeID}, nil
	}
	if task.Status != model.ExportStatusPending {
		return ImportResult{}, domain.ErrExportNotFound
	}
	node, err := model.NewFile(cmd.TargetNamespaceID, cmd.OwnerID, cmd.Name, cmd.TargetParentID)
	if err != nil {
		return ImportResult{}, err
	}
	if node.ID == task.SourceNodeID {
		node.ID = uuid.New()
	}
	node.CurrentRevisionID = nil
	node.Classification = classification
	if err := h.nodes.CreateNode(ctx, node); err != nil {
		return ImportResult{}, err
	}
	if err := h.store.MarkExportImported(ctx, task.ID, node.ID); err != nil {
		return ImportResult{}, err
	}
	return ImportResult{NodeID: node.ID, SourceNodeID: task.SourceNodeID}, nil
}

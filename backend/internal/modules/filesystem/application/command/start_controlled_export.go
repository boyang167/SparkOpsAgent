package command

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type StartControlledExport struct {
	ID                   uuid.UUID
	ActorID              uuid.UUID
	SourceOrganizationID uuid.UUID
	TargetOrganizationID uuid.UUID
	SourceWorkspaceID    uuid.UUID
	TargetWorkspaceID    uuid.UUID
	SourceNodeID         uuid.UUID
}

type ExportNodeReader interface {
	GetNodeByID(context.Context, uuid.UUID) (*model.Node, error)
}

type StartControlledExportHandler struct {
	store repository.CapabilityRepository
	audit audit.Logger
	nodes ExportNodeReader
}

func NewStartControlledExportHandler(store repository.CapabilityRepository, logger audit.Logger, nodes ExportNodeReader) *StartControlledExportHandler {
	if store == nil || nodes == nil {
		panic("controlled export requires store and node reader")
	}
	return &StartControlledExportHandler{store: store, audit: logger, nodes: nodes}
}

func (h *StartControlledExportHandler) Handle(ctx context.Context, cmd StartControlledExport) (model.ExportTask, error) {
	if cmd.ID == uuid.Nil {
		cmd.ID = uuid.New()
	}
	if existing, err := h.store.GetExportTask(ctx, cmd.ID); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrExportNotFound) {
		return model.ExportTask{}, err
	}
	if cmd.SourceOrganizationID == uuid.Nil || cmd.TargetOrganizationID == uuid.Nil || cmd.SourceOrganizationID == cmd.TargetOrganizationID || cmd.SourceNodeID == uuid.Nil || cmd.ActorID == uuid.Nil {
		return model.ExportTask{}, domain.ErrControlledExportScope
	}
	source, err := h.nodes.GetNodeByID(ctx, cmd.SourceNodeID)
	if err != nil {
		return model.ExportTask{}, err
	}
	switch source.Classification {
	case model.TransferClassificationPublic, model.TransferClassificationInternal,
		model.TransferClassificationConfidential, model.TransferClassificationRestricted:
	default:
		return model.ExportTask{}, domain.ErrPersistence
	}
	task := model.ExportTask{
		ID: cmd.ID, SourceOrganizationID: cmd.SourceOrganizationID, TargetOrganizationID: cmd.TargetOrganizationID,
		SourceWorkspaceID: cmd.SourceWorkspaceID, TargetWorkspaceID: cmd.TargetWorkspaceID,
		SourceNodeID: cmd.SourceNodeID, Status: model.ExportStatusPending, ActorID: cmd.ActorID,
		Classification: source.Classification,
	}
	if err := h.store.SaveExportTask(ctx, task); err != nil {
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			return h.store.GetExportTask(ctx, cmd.ID)
		}
		return model.ExportTask{}, err
	}
	if err := h.auditBoth(ctx, task); err != nil {
		return model.ExportTask{}, err
	}
	return task, nil
}

func (h *StartControlledExportHandler) auditBoth(ctx context.Context, task model.ExportTask) error {
	if h.audit == nil {
		return nil
	}
	scopes := []struct {
		organizationID uuid.UUID
		workspaceID    uuid.UUID
	}{
		{organizationID: task.SourceOrganizationID, workspaceID: task.SourceWorkspaceID},
		{organizationID: task.TargetOrganizationID, workspaceID: task.TargetWorkspaceID},
	}
	for _, scope := range scopes {
		if err := h.audit.Log(ctx, audit.Entry{
			ActorID: kernel.MemberID{UUID: task.ActorID}, OrgID: &kernel.OrgID{UUID: scope.organizationID},
			Action: "filesystem.export.started", ResourceType: "filesystem", ResourceID: task.SourceNodeID.String(),
			Detail: map[string]any{
				"export_id": task.ID.String(), "source_organization_id": task.SourceOrganizationID.String(),
				"target_organization_id": task.TargetOrganizationID.String(),
				"workspace_id":           scope.workspaceID.String(), "decision": "ALLOW",
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

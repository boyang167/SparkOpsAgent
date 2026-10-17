package outbound

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	OperationTypeProjectUndeploy = "workspace-app.project.undeploy.v1"
	OperationTypePipelineDisable = "workspace-app.pipeline.disable.v1"
)

type ProjectUndeployPayload struct {
	EventID   string `json:"event_id"`
	ProjectID string `json:"project_id"`
	MemberID  string `json:"member_id"`
}

type PipelineDisablePayload struct {
	EventID     string `json:"event_id"`
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id"`
}

type PipelineDisabler interface {
	DisablePipeline(context.Context, kernel.WorkspaceID, kernel.ProjectID) error
}

func NewPipelineDisableHandler(disabler func() PipelineDisabler) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload PipelineDisablePayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode pipeline disable operation: %w", err))
		}
		workspaceID, err := kernel.ParseWorkspaceID(payload.WorkspaceID)
		if err != nil {
			return messaging.Permanent(err)
		}
		projectID, err := kernel.ParseProjectID(payload.ProjectID)
		if err != nil {
			return messaging.Permanent(err)
		}
		target := disabler()
		if target == nil {
			return fmt.Errorf("project teardown unavailable")
		}
		return target.DisablePipeline(ctx, workspaceID, projectID)
	}
}

type ArchiveAdvancer interface {
	Advance(context.Context, *model.Project, kernel.MemberID) error
}

func NewProjectUndeployHandler(repo repository.ProjectRepository, coordinator ArchiveAdvancer) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload ProjectUndeployPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode project undeploy operation: %w", err))
		}
		projectID, err := kernel.ParseProjectID(payload.ProjectID)
		if err != nil {
			return messaging.Permanent(err)
		}
		memberID, err := kernel.ParseMemberID(payload.MemberID)
		if err != nil {
			return messaging.Permanent(err)
		}
		project, err := repo.FindByID(ctx, projectID)
		if err != nil || !project.IsArchiving() {
			return err
		}
		return coordinator.Advance(ctx, project, memberID)
	}
}

package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	OperationTypeProjectUndeploy = "workspace-app.project.undeploy.v1"
	OperationTypePipelineDisable = "workspace-app.pipeline.disable.v1"
)

type ProjectUndeployPayload struct {
	EventID       string `json:"event_id"`
	ProjectID     string `json:"project_id"`
	MemberID      string `json:"member_id"`
	Generation    string `json:"generation"`
	AllowUndeploy *bool  `json:"allow_undeploy,omitempty"`
}

type PipelineDisablePayload struct {
	EventID     string `json:"event_id"`
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id"`
	Generation  string `json:"generation"`
}

type PipelineDisabler interface {
	DisablePipeline(context.Context, kernel.WorkspaceID, kernel.ProjectID) error
	PipelineDisabled(context.Context, kernel.WorkspaceID, kernel.ProjectID) (bool, error)
}

func NewPipelineDisableHandler(
	disabler func() PipelineDisabler,
	repo repository.ProjectTerminalRepository,
	tx postgres.Transactor,
) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload PipelineDisablePayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode pipeline disable operation: %w", err))
		}
		if strings.TrimSpace(payload.Generation) == "" {
			return nil
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
		project, err := repo.FindByID(ctx, projectID)
		if err != nil {
			return err
		}
		if project.Status == model.ProjectStatusArchived || !project.IsArchiving() ||
			project.ArchiveGeneration != payload.Generation {
			return nil
		}
		disabled, err := target.PipelineDisabled(ctx, workspaceID, projectID)
		if err != nil {
			return err
		}
		current, err := projectGenerationCurrent(ctx, tx, repo, projectID, payload.Generation)
		if err != nil || !current {
			return err
		}
		if !disabled {
			if err := target.DisablePipeline(ctx, workspaceID, projectID); err != nil {
				return err
			}
			current, err = projectGenerationCurrent(ctx, tx, repo, projectID, payload.Generation)
			if err != nil || !current {
				return err
			}
			disabled, err = target.PipelineDisabled(ctx, workspaceID, projectID)
			if err != nil {
				return err
			}
			current, err = projectGenerationCurrent(ctx, tx, repo, projectID, payload.Generation)
			if err != nil || !current {
				return err
			}
		}
		if !disabled {
			return fmt.Errorf("pipeline remains enabled after disable")
		}
		return tx.WithinTransaction(ctx, func(txCtx context.Context) error {
			return transitionProjectTerminal(txCtx, repo, projectID, payload.Generation, model.ProjectStatusArchived)
		})
	}
}

func NewProjectUndeployHandler(
	repo repository.ProjectTerminalRepository,
	teardown func() appctx.ProjectTeardown,
	operations messaging.OperationWriter,
	tx postgres.Transactor,
) messaging.OperationHandler {
	return func(ctx context.Context, operation messaging.Operation) error {
		var payload ProjectUndeployPayload
		if err := json.Unmarshal(operation.Payload, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("decode project undeploy operation: %w", err))
		}
		if strings.TrimSpace(payload.Generation) == "" {
			return nil
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
		if project.ArchiveGeneration != payload.Generation {
			return nil
		}
		target := teardown()
		if target == nil {
			return fmt.Errorf("project teardown unavailable")
		}
		online, err := target.OnlineClusters(ctx, project.WorkspaceID, project.ID)
		if err != nil {
			return err
		}
		current, err := projectGenerationCurrent(ctx, tx, repo, projectID, payload.Generation)
		if err != nil || !current {
			return err
		}
		allowUndeploy := true
		if payload.AllowUndeploy != nil {
			allowUndeploy = *payload.AllowUndeploy
		}
		if len(online) > 0 && !allowUndeploy {
			return tx.WithinTransaction(ctx, func(txCtx context.Context) error {
				return transitionProjectTerminal(txCtx, repo, projectID, payload.Generation, model.ProjectStatusActive)
			})
		}
		active, err := target.HasActiveRun(ctx, project.ID)
		if err != nil {
			return err
		}
		current, err = projectGenerationCurrent(ctx, tx, repo, projectID, payload.Generation)
		if err != nil || !current {
			return err
		}
		if active {
			return errors.New("project undeploy is still active")
		}
		if len(online) > 0 {
			if allowUndeploy {
				_, err = target.StartUndeploy(ctx, appctx.UndeployRequest{
					WorkspaceID:       project.WorkspaceID,
					ProjectID:         project.ID,
					MemberID:          memberID,
					ClusterID:         online[0],
					ArchiveGeneration: payload.Generation,
				})
				var domainErr *kernel.DomainError
				if err != nil && !(errors.As(err, &domainErr) && domainErr.Code() == "DELIVERY_DEPLOYMENT_RUNNING") {
					return err
				}
				current, err = projectGenerationCurrent(ctx, tx, repo, projectID, payload.Generation)
				if err != nil || !current {
					return err
				}
			}
			return errors.New("project remains online after undeploy reconciliation")
		}
		return tx.WithinTransaction(ctx, func(txCtx context.Context) error {
			txCtx = events.WithMetadata(txCtx, events.Metadata{EventID: operation.EventID, Subscriber: operation.Owner})
			current, err := repo.FindByID(txCtx, projectID)
			if err != nil || !current.IsArchiving() {
				return err
			}
			if current.ArchiveGeneration != payload.Generation {
				return nil
			}
			return operations.Schedule(txCtx, messaging.OperationRequest{
				Owner:          operation.Owner,
				OperationType:  OperationTypePipelineDisable,
				IdempotencyKey: operation.EventID + ":" + operation.Owner + ":" + OperationTypePipelineDisable + ":" + projectID.String() + "/" + payload.Generation,
				Payload: PipelineDisablePayload{
					EventID: operation.EventID, WorkspaceID: current.WorkspaceID.String(), ProjectID: current.ID.String(),
					Generation: payload.Generation,
				},
			})
		})
	}
}

func projectGenerationCurrent(
	ctx context.Context,
	tx postgres.Transactor,
	repo repository.ProjectTerminalRepository,
	projectID kernel.ProjectID,
	generation string,
) (bool, error) {
	var current bool
	err := tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		project, err := repo.FindByID(txCtx, projectID)
		if err != nil {
			return err
		}
		current = project.IsArchiving() && project.ArchiveGeneration == generation
		return nil
	})
	return current, err
}

func transitionProjectTerminal(
	ctx context.Context,
	repo repository.ProjectTerminalRepository,
	projectID kernel.ProjectID,
	generation string,
	target model.ProjectStatus,
) error {
	updated, err := repo.TransitionArchiving(ctx, projectID, generation, target)
	if err != nil || updated {
		return err
	}
	current, err := repo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}
	if current.Status == target {
		return nil
	}
	switch current.Status {
	case model.ProjectStatusActive, model.ProjectStatusArchived:
		return nil
	default:
		return fmt.Errorf("project terminal transition conflicted with status %q", current.Status)
	}
}

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
	EventID       string `json:"event_id"`
	WorkspaceID   string `json:"workspace_id"`
	ProjectID     string `json:"project_id"`
	Generation    string `json:"generation"`
	AllowUndeploy *bool  `json:"allow_undeploy,omitempty"`
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
		proceed, err := claimPipelineFinalizing(ctx, tx, repo, projectID, payload.Generation)
		if err != nil || !proceed {
			return err
		}
		target := disabler()
		if target == nil {
			return fmt.Errorf("project teardown unavailable")
		}
		disabled, err := target.PipelineDisabled(ctx, workspaceID, projectID)
		if err != nil {
			return err
		}
		current, err := projectPhaseCurrent(
			ctx, tx, repo, projectID, payload.Generation, model.ArchivePhaseFinalizing,
		)
		if err != nil || !current {
			return err
		}
		if !disabled {
			if err := target.DisablePipeline(ctx, workspaceID, projectID); err != nil {
				return err
			}
			current, err = projectPhaseCurrent(
				ctx, tx, repo, projectID, payload.Generation, model.ArchivePhaseFinalizing,
			)
			if err != nil || !current {
				return err
			}
			disabled, err = target.PipelineDisabled(ctx, workspaceID, projectID)
			if err != nil {
				return err
			}
			current, err = projectPhaseCurrent(
				ctx, tx, repo, projectID, payload.Generation, model.ArchivePhaseFinalizing,
			)
			if err != nil || !current {
				return err
			}
		}
		if !disabled {
			return fmt.Errorf("pipeline remains enabled after disable")
		}
		return tx.WithinTransaction(ctx, func(txCtx context.Context) error {
			return transitionProjectTerminal(
				txCtx, repo, projectID, payload.Generation,
				model.ArchivePhaseFinalizing, model.ProjectStatusArchived,
			)
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
		allowUndeploy, current, err := currentArchiveIntent(
			ctx, tx, repo, projectID, payload.Generation,
		)
		if err != nil || !current {
			return err
		}
		if !allowUndeploy {
			return handleArchiveWithoutUndeploy(
				ctx, operation, repo, teardown, operations, tx, projectID, payload.Generation,
			)
		}
		project, proceed, err := claimUndeploy(ctx, tx, repo, projectID, payload.Generation)
		if err != nil || !proceed {
			return err
		}
		if project.ArchiveAllowUndeploy == nil || !*project.ArchiveAllowUndeploy {
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
		current, err = projectPhaseOneOf(
			ctx, tx, repo, projectID, payload.Generation,
			model.ArchivePhaseUndeployClaimed, model.ArchivePhaseUndeploying,
		)
		if err != nil || !current {
			return err
		}
		active, err := target.HasActiveRun(ctx, project.ID)
		if err != nil {
			return err
		}
		current, err = projectPhaseOneOf(
			ctx, tx, repo, projectID, payload.Generation,
			model.ArchivePhaseUndeployClaimed, model.ArchivePhaseUndeploying,
		)
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
				current, err = projectPhaseOneOf(
					ctx, tx, repo, projectID, payload.Generation,
					model.ArchivePhaseUndeployClaimed, model.ArchivePhaseUndeploying,
				)
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
			if current.ArchiveGeneration != payload.Generation ||
				(current.ArchivePhase != model.ArchivePhaseUndeployClaimed &&
					current.ArchivePhase != model.ArchivePhaseUndeploying) {
				return nil
			}
			advanced, err := repo.TransitionArchivePhase(
				txCtx, projectID, payload.Generation, current.ArchivePhase, model.ArchivePhasePipelinePending,
			)
			if err != nil {
				return err
			}
			if !advanced {
				return errors.New("archive phase transition to pipeline pending conflicted")
			}
			return operations.Schedule(txCtx, messaging.OperationRequest{
				Owner:          operation.Owner,
				OperationType:  OperationTypePipelineDisable,
				IdempotencyKey: operation.EventID + ":" + operation.Owner + ":" + OperationTypePipelineDisable + ":" + projectID.String() + "/" + payload.Generation,
				Payload: PipelineDisablePayload{
					EventID: operation.EventID, WorkspaceID: current.WorkspaceID.String(), ProjectID: current.ID.String(),
					Generation: payload.Generation, AllowUndeploy: current.ArchiveAllowUndeploy,
				},
			})
		})
	}
}

func handleArchiveWithoutUndeploy(
	ctx context.Context,
	operation messaging.Operation,
	repo repository.ProjectTerminalRepository,
	teardown func() appctx.ProjectTeardown,
	operations messaging.OperationWriter,
	tx postgres.Transactor,
	projectID kernel.ProjectID,
	generation string,
) error {
	project, current, err := projectPendingIntent(ctx, tx, repo, projectID, generation, false)
	if err != nil || !current {
		return err
	}
	target := teardown()
	if target == nil {
		return fmt.Errorf("project teardown unavailable")
	}
	online, err := target.OnlineClusters(ctx, project.WorkspaceID, project.ID)
	if err != nil {
		return err
	}
	_, current, err = projectPendingIntent(ctx, tx, repo, projectID, generation, false)
	if err != nil || !current {
		return err
	}
	active, err := target.HasActiveRun(ctx, project.ID)
	if err != nil {
		return err
	}
	_, current, err = projectPendingIntent(ctx, tx, repo, projectID, generation, false)
	if err != nil || !current {
		return err
	}
	if len(online) > 0 || active {
		return tx.WithinTransaction(ctx, func(txCtx context.Context) error {
			return transitionProjectTerminal(
				txCtx, repo, projectID, generation,
				model.ArchivePhasePending, model.ProjectStatusActive,
			)
		})
	}
	return tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		currentProject, err := repo.FindByID(txCtx, projectID)
		if err != nil {
			return err
		}
		if !archiveIntentMatches(currentProject, generation, model.ArchivePhasePending, false) {
			return nil
		}
		advanced, err := repo.TransitionArchivePhase(
			txCtx, projectID, generation, model.ArchivePhasePending, model.ArchivePhasePipelinePending,
		)
		if err != nil {
			return err
		}
		if !advanced {
			return errors.New("archive phase transition to pipeline pending conflicted")
		}
		return operations.Schedule(txCtx, messaging.OperationRequest{
			Owner:          operation.Owner,
			OperationType:  OperationTypePipelineDisable,
			IdempotencyKey: operation.EventID + ":" + operation.Owner + ":" + OperationTypePipelineDisable + ":" + projectID.String() + "/" + generation,
			Payload: PipelineDisablePayload{
				EventID: operation.EventID, WorkspaceID: currentProject.WorkspaceID.String(),
				ProjectID: projectID.String(), Generation: generation,
				AllowUndeploy: currentProject.ArchiveAllowUndeploy,
			},
		})
	})
}

func projectPendingIntent(
	ctx context.Context,
	tx postgres.Transactor,
	repo repository.ProjectTerminalRepository,
	projectID kernel.ProjectID,
	generation string,
	allowUndeploy bool,
) (*model.Project, bool, error) {
	var project *model.Project
	var current bool
	err := tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		var err error
		project, err = repo.FindByID(txCtx, projectID)
		if err != nil {
			return err
		}
		current = archiveIntentMatches(project, generation, model.ArchivePhasePending, allowUndeploy)
		return nil
	})
	return project, current, err
}

func currentArchiveIntent(
	ctx context.Context,
	tx postgres.Transactor,
	repo repository.ProjectTerminalRepository,
	projectID kernel.ProjectID,
	generation string,
) (bool, bool, error) {
	var allowUndeploy bool
	var current bool
	err := tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		project, err := repo.FindByID(txCtx, projectID)
		if err != nil {
			return err
		}
		if !project.IsArchiving() ||
			project.ArchiveGeneration != generation ||
			project.ArchiveAllowUndeploy == nil {
			return nil
		}
		allowUndeploy = *project.ArchiveAllowUndeploy
		current = true
		return nil
	})
	return allowUndeploy, current, err
}

func archiveIntentMatches(
	project *model.Project,
	generation string,
	phase model.ArchivePhase,
	allowUndeploy bool,
) bool {
	return project.IsArchiving() &&
		project.ArchiveGeneration == generation &&
		project.ArchivePhase == phase &&
		project.ArchiveAllowUndeploy != nil &&
		*project.ArchiveAllowUndeploy == allowUndeploy
}

func claimUndeploy(
	ctx context.Context,
	tx postgres.Transactor,
	repo repository.ProjectTerminalRepository,
	projectID kernel.ProjectID,
	generation string,
) (*model.Project, bool, error) {
	var project *model.Project
	var proceed bool
	err := tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		current, err := repo.FindByID(txCtx, projectID)
		if err != nil {
			return err
		}
		if !current.IsArchiving() || current.ArchiveGeneration != generation {
			return nil
		}
		switch current.ArchivePhase {
		case model.ArchivePhasePending:
			claimed, err := repo.TransitionArchivePhase(
				txCtx, projectID, generation,
				model.ArchivePhasePending, model.ArchivePhaseUndeployClaimed,
			)
			if err != nil {
				return err
			}
			if !claimed {
				_, err = repo.FindByID(txCtx, projectID)
				if err != nil {
					return err
				}
				return nil
			}
			current.ArchivePhase = model.ArchivePhaseUndeployClaimed
			proceed = true
		case model.ArchivePhaseUndeployClaimed, model.ArchivePhaseUndeploying:
			proceed = true
		}
		project = current
		return nil
	})
	return project, proceed, err
}

func claimPipelineFinalizing(
	ctx context.Context,
	tx postgres.Transactor,
	repo repository.ProjectTerminalRepository,
	projectID kernel.ProjectID,
	generation string,
) (bool, error) {
	var proceed bool
	err := tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		current, err := repo.FindByID(txCtx, projectID)
		if err != nil {
			return err
		}
		if !current.IsArchiving() || current.ArchiveGeneration != generation {
			return nil
		}
		switch current.ArchivePhase {
		case model.ArchivePhasePipelinePending:
			claimed, err := repo.TransitionArchivePhase(
				txCtx, projectID, generation,
				model.ArchivePhasePipelinePending, model.ArchivePhasePipelineClaimed,
			)
			if err != nil {
				return err
			}
			if !claimed {
				_, err = repo.FindByID(txCtx, projectID)
				return err
			}
			finalizing, err := repo.TransitionArchivePhase(
				txCtx, projectID, generation,
				model.ArchivePhasePipelineClaimed, model.ArchivePhaseFinalizing,
			)
			if err != nil {
				return err
			}
			if !finalizing {
				_, err = repo.FindByID(txCtx, projectID)
				return err
			}
			proceed = true
		case model.ArchivePhasePipelineClaimed:
			finalizing, err := repo.TransitionArchivePhase(
				txCtx, projectID, generation,
				model.ArchivePhasePipelineClaimed, model.ArchivePhaseFinalizing,
			)
			if err != nil {
				return err
			}
			if !finalizing {
				_, err = repo.FindByID(txCtx, projectID)
				return err
			}
			proceed = true
		case model.ArchivePhaseFinalizing:
			proceed = true
		}
		return nil
	})
	return proceed, err
}

func projectPhaseCurrent(
	ctx context.Context,
	tx postgres.Transactor,
	repo repository.ProjectTerminalRepository,
	projectID kernel.ProjectID,
	generation string,
	phase model.ArchivePhase,
) (bool, error) {
	var current bool
	err := tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		project, err := repo.FindByID(txCtx, projectID)
		if err != nil {
			return err
		}
		current = project.IsArchiving() &&
			project.ArchiveGeneration == generation &&
			project.ArchivePhase == phase
		return nil
	})
	return current, err
}

func projectPhaseOneOf(
	ctx context.Context,
	tx postgres.Transactor,
	repo repository.ProjectTerminalRepository,
	projectID kernel.ProjectID,
	generation string,
	phases ...model.ArchivePhase,
) (bool, error) {
	var current bool
	err := tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		project, err := repo.FindByID(txCtx, projectID)
		if err != nil {
			return err
		}
		if !project.IsArchiving() || project.ArchiveGeneration != generation {
			return nil
		}
		for _, phase := range phases {
			if project.ArchivePhase == phase {
				current = true
				break
			}
		}
		return nil
	})
	return current, err
}

func transitionProjectTerminal(
	ctx context.Context,
	repo repository.ProjectTerminalRepository,
	projectID kernel.ProjectID,
	generation string,
	expected model.ArchivePhase,
	target model.ProjectStatus,
) error {
	updated, err := repo.TransitionArchiving(ctx, projectID, generation, expected, target)
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

package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	workspaceoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	undeployRunType            = "undeploy"
	deployRunSucceeded         = "succeeded"
	deploymentRunningErrorCode = "DELIVERY_DEPLOYMENT_RUNNING"
)

var errTeardownUnavailable = errors.New("project teardown is not configured")

type archiveRunGenerationLookup interface {
	ArchiveGeneration(context.Context, kernel.DeploymentRunID) (string, error)
}

// ArchiveCoordinator takes an archiving project offline one cluster at a time,
// because delivery allows only one active run per project.
type ArchiveCoordinator struct {
	repo       repository.ProjectRepository
	teardown   func() appctx.ProjectTeardown
	warn       func(string, error)
	operations messaging.OperationWriter
	tx         postgres.Transactor
}

func (c *ArchiveCoordinator) UseTransactor(tx postgres.Transactor) {
	c.tx = tx
}

func NewArchiveCoordinator(
	repo repository.ProjectRepository,
	teardown func() appctx.ProjectTeardown,
	warn func(string, error),
	writers ...messaging.OperationWriter,
) *ArchiveCoordinator {
	if warn == nil {
		warn = func(string, error) {}
	}
	coordinator := &ArchiveCoordinator{repo: repo, teardown: teardown, warn: warn}
	if len(writers) > 0 {
		coordinator.operations = writers[0]
	}
	return coordinator
}

func (c *ArchiveCoordinator) port() (appctx.ProjectTeardown, error) {
	if c.teardown == nil {
		return nil, errTeardownUnavailable
	}
	teardown := c.teardown()
	if teardown == nil {
		return nil, errTeardownUnavailable
	}
	return teardown, nil
}

func (c *ArchiveCoordinator) Advance(ctx context.Context, project *model.Project, memberID kernel.MemberID) error {
	return c.AdvanceWithTeardown(ctx, project, memberID, true)
}

func (c *ArchiveCoordinator) AdvanceWithTeardown(
	ctx context.Context,
	project *model.Project,
	memberID kernel.MemberID,
	allowUndeploy bool,
) error {
	teardown, err := c.port()
	if err != nil {
		return err
	}
	online, err := teardown.OnlineClusters(ctx, project.WorkspaceID, project.ID)
	if err != nil {
		return err
	}
	if len(online) == 0 {
		return c.finish(ctx, teardown, project)
	}
	if !allowUndeploy {
		if err := project.AbortArchiving(); err != nil {
			return err
		}
		return c.repo.Update(ctx, project)
	}
	active, err := teardown.HasActiveRun(ctx, project.ID)
	if err != nil || active {
		return err
	}
	_, err = teardown.StartUndeploy(ctx, appctx.UndeployRequest{
		WorkspaceID: project.WorkspaceID, ProjectID: project.ID, MemberID: memberID, ClusterID: online[0],
		ArchiveGeneration: project.ArchiveGeneration,
	})
	if isDeploymentRunning(err) {
		return nil
	}
	return err
}

func (c *ArchiveCoordinator) finish(ctx context.Context, teardown appctx.ProjectTeardown, project *model.Project) error {
	if c.operations != nil {
		metadata, ok := events.MetadataFromContext(ctx)
		if !ok {
			return errors.New("pipeline disable operation requires event metadata")
		}
		return c.operations.Schedule(ctx, messaging.OperationRequest{
			Owner:          metadata.Subscriber,
			OperationType:  workspaceoutbound.OperationTypePipelineDisable,
			IdempotencyKey: metadata.EventID + ":" + metadata.Subscriber + ":" + workspaceoutbound.OperationTypePipelineDisable + ":" + project.ID.String() + "/" + project.ArchiveGeneration,
			Payload: workspaceoutbound.PipelineDisablePayload{
				EventID: metadata.EventID, WorkspaceID: project.WorkspaceID.String(), ProjectID: project.ID.String(),
				Generation: project.ArchiveGeneration,
			},
		})
	}
	disabled, err := teardown.PipelineDisabled(ctx, project.WorkspaceID, project.ID)
	if err != nil {
		return err
	}
	if !disabled {
		if err := teardown.DisablePipeline(ctx, project.WorkspaceID, project.ID); err != nil {
			return err
		}
		disabled, err = teardown.PipelineDisabled(ctx, project.WorkspaceID, project.ID)
		if err != nil {
			return err
		}
	}
	if !disabled {
		return fmt.Errorf("pipeline remains enabled after disable")
	}
	if err := project.Archive(); err != nil {
		return err
	}
	return c.repo.Update(ctx, project)
}

func (c *ArchiveCoordinator) HandleDeployCompleted(ctx context.Context, e events.Event) error {
	ev, ok := e.(deliveryevent.DeployCompleted)
	if !ok || ev.RunType != undeployRunType {
		return nil
	}
	teardown, err := c.port()
	if err != nil {
		return err
	}
	generations, ok := teardown.(archiveRunGenerationLookup)
	if !ok {
		return errors.New("project teardown cannot resolve archive generation")
	}
	generation, err := generations.ArchiveGeneration(ctx, ev.RunID)
	if err != nil {
		return err
	}
	if generation == "" {
		return nil
	}
	project, err := c.repo.FindByID(ctx, ev.ProjectID)
	if err != nil {
		return err
	}
	if !project.IsArchiving() || project.ArchiveGeneration != generation {
		return nil
	}
	if ev.Status != deployRunSucceeded {
		if err := c.abortGeneration(ctx, project.ID, generation); err != nil {
			return err
		}
		c.warn("archive aborted: undeploy "+ev.RunID.String()+" "+ev.Status, nil)
		return nil
	}
	if c.operations != nil {
		metadata, ok := events.MetadataFromContext(ctx)
		if !ok {
			return errors.New("archive operation requires event metadata")
		}
		return c.operations.Schedule(ctx, messaging.OperationRequest{
			Owner: metadata.Subscriber, OperationType: workspaceoutbound.OperationTypeProjectUndeploy,
			IdempotencyKey: metadata.EventID + ":" + metadata.Subscriber + ":" + workspaceoutbound.OperationTypeProjectUndeploy + ":" + project.ID.String() + "/" + generation,
			Payload: workspaceoutbound.ProjectUndeployPayload{
				EventID: metadata.EventID, ProjectID: project.ID.String(), MemberID: ev.CreatedBy.String(),
				AllowUndeploy: boolPointer(true), Generation: generation,
			},
		})
	}
	if err := c.Advance(ctx, project, ev.CreatedBy); err != nil {
		c.warn("advance archive failed", err)
		if abortErr := c.abortGeneration(ctx, project.ID, generation); abortErr != nil {
			return errors.Join(err, abortErr)
		}
		return err
	}
	return nil
}

func (c *ArchiveCoordinator) abortGeneration(
	ctx context.Context, projectID kernel.ProjectID, generation string,
) error {
	terminal, ok := c.repo.(repository.ProjectTerminalRepository)
	if !ok {
		return errors.New("archive completion requires terminal transition repository")
	}
	transitioned, err := terminal.TransitionArchiving(ctx, projectID, generation, model.ProjectStatusActive)
	if err != nil || transitioned {
		return err
	}
	current, err := c.repo.FindByID(ctx, projectID)
	if err != nil {
		return err
	}
	if current.ArchiveGeneration != generation ||
		current.Status == model.ProjectStatusActive ||
		current.Status == model.ProjectStatusArchived {
		return nil
	}
	return errors.New("archive abort transition conflicted")
}

// ResumeAll picks up archiving projects whose completion event was missed, e.g. after a restart.
func (c *ArchiveCoordinator) ResumeAll(ctx context.Context) error {
	projects, err := c.repo.ListByStatus(ctx, model.ProjectStatusArchiving)
	if err != nil {
		return err
	}
	var errs []error
	for _, project := range projects {
		if c.operations == nil {
			if err := c.Advance(ctx, project, project.CreatedBy); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if c.tx == nil {
			errs = append(errs, errors.New("archive resumer requires transaction"))
			continue
		}
		err := c.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
			current, err := c.repo.FindByID(txCtx, project.ID)
			if err != nil || !current.IsArchiving() {
				return err
			}
			if current.ArchiveGeneration == "" {
				terminal, ok := c.repo.(repository.ProjectTerminalRepository)
				if !ok {
					return errors.New("archive resumer requires generation repository")
				}
				generation := uuid.NewString()
				initialized, err := terminal.InitializeArchiveGeneration(txCtx, current.ID, generation)
				if err != nil {
					return err
				}
				if initialized {
					current.ArchiveGeneration = generation
				} else {
					current, err = c.repo.FindByID(txCtx, current.ID)
					if err != nil || !current.IsArchiving() || current.ArchiveGeneration == "" {
						return err
					}
				}
			}
			eventID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(
				"workspace-app.archive-resumer:"+current.ID.String()+":"+current.ArchiveGeneration,
			)).String()
			txCtx = events.WithMetadata(txCtx, events.Metadata{
				EventID: eventID, Subscriber: "workspace-app.archive-resumer",
			})
			return c.operations.Schedule(txCtx, messaging.OperationRequest{
				Owner: "workspace-app.archive-resumer", OperationType: workspaceoutbound.OperationTypeProjectUndeploy,
				IdempotencyKey: eventID + ":workspace-app.archive-resumer:" + workspaceoutbound.OperationTypeProjectUndeploy + ":" + current.ID.String() + "/" + current.ArchiveGeneration,
				Payload: workspaceoutbound.ProjectUndeployPayload{
					EventID: eventID, ProjectID: current.ID.String(), MemberID: current.CreatedBy.String(),
					AllowUndeploy: boolPointer(true), Generation: current.ArchiveGeneration,
				},
			})
		})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func isDeploymentRunning(err error) bool {
	var domainErr *kernel.DomainError
	return errors.As(err, &domainErr) && domainErr.Code() == deploymentRunningErrorCode
}

func boolPointer(value bool) *bool {
	return &value
}

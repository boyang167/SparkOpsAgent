package command

import (
	"context"
	"errors"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	undeployRunType            = "undeploy"
	deployRunSucceeded         = "succeeded"
	deploymentRunningErrorCode = "DELIVERY_DEPLOYMENT_RUNNING"
)

var errTeardownUnavailable = errors.New("project teardown is not configured")

// ArchiveCoordinator takes an archiving project offline one cluster at a time,
// because delivery allows only one active run per project.
type ArchiveCoordinator struct {
	repo     repository.ProjectRepository
	teardown func() appctx.ProjectTeardown
	warn     func(string, error)
}

func NewArchiveCoordinator(
	repo repository.ProjectRepository,
	teardown func() appctx.ProjectTeardown,
	warn func(string, error),
) *ArchiveCoordinator {
	if warn == nil {
		warn = func(string, error) {}
	}
	return &ArchiveCoordinator{repo: repo, teardown: teardown, warn: warn}
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
	active, err := teardown.HasActiveRun(ctx, project.ID)
	if err != nil || active {
		return err
	}
	_, err = teardown.StartUndeploy(ctx, appctx.UndeployRequest{
		WorkspaceID: project.WorkspaceID, ProjectID: project.ID, MemberID: memberID, ClusterID: online[0],
	})
	if isDeploymentRunning(err) {
		return nil
	}
	return err
}

func (c *ArchiveCoordinator) finish(ctx context.Context, teardown appctx.ProjectTeardown, project *model.Project) error {
	if err := teardown.DisablePipeline(ctx, project.WorkspaceID, project.ID); err != nil {
		c.warn("disable jenkins job failed during archive", err)
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
	if messaging.IsInboxConsumerContext(ctx) {
		return messaging.ErrOperationOutboxRequired
	}
	project, err := c.repo.FindByID(ctx, ev.ProjectID)
	if err != nil || !project.IsArchiving() {
		return nil
	}
	if ev.Status != deployRunSucceeded {
		if err := project.AbortArchiving(); err != nil {
			return err
		}
		c.warn("archive aborted: undeploy "+ev.RunID.String()+" "+ev.Status, nil)
		return c.repo.Update(ctx, project)
	}
	if err := c.Advance(ctx, project, ev.CreatedBy); err != nil {
		c.warn("advance archive failed", err)
		if abortErr := project.AbortArchiving(); abortErr == nil {
			return c.repo.Update(ctx, project)
		}
		return err
	}
	return nil
}

// ResumeAll picks up archiving projects whose completion event was missed, e.g. after a restart.
func (c *ArchiveCoordinator) ResumeAll(ctx context.Context) error {
	projects, err := c.repo.ListByStatus(ctx, model.ProjectStatusArchiving)
	if err != nil {
		return err
	}
	var errs []error
	for _, project := range projects {
		if err := c.Advance(ctx, project, project.CreatedBy); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func isDeploymentRunning(err error) bool {
	var domainErr *kernel.DomainError
	return errors.As(err, &domainErr) && domainErr.Code() == deploymentRunningErrorCode
}

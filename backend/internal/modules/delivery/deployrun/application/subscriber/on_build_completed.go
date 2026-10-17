package subscriber

import (
	"context"
	"errors"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployrepo "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type runPublisher interface {
	PublishRun(context.Context, *model.DeploymentRun) error
}

type OnBuildCompleted struct {
	repo      deployrepo.DeploymentRunRepository
	publisher runPublisher
	clock     func() time.Time
}

func NewOnBuildCompleted(repo deployrepo.DeploymentRunRepository, publisher runPublisher) *OnBuildCompleted {
	return &OnBuildCompleted{
		repo: repo, publisher: publisher,
		clock: func() time.Time { return time.Now().UTC() },
	}
}

func (s *OnBuildCompleted) Handle(ctx context.Context, event events.Event) error {
	completed, ok := event.(deliveryevent.BuildCompleted)
	if !ok {
		return nil
	}
	if messaging.IsInboxConsumerContext(ctx) {
		return messaging.ErrOperationOutboxRequired
	}
	run, err := s.repo.FindByBuildID(ctx, completed.BuildID)
	if errors.Is(err, domain.ErrDeploymentRunNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if run.BuildID == nil || *run.BuildID != completed.BuildID ||
		run.ProjectID != completed.ProjectID || run.WorkspaceID != completed.WorkspaceID {
		return nil
	}

	if run.Status != model.StatusBuilding {
		return nil
	}
	switch buildmodel.BuildStatus(completed.Status) {
	case buildmodel.BuildStatusFailed:
		run.Fail("build completed with status "+completed.Status, s.clock())
		if err := s.repo.Update(ctx, run); err != nil {
			return s.resolveConcurrent(ctx, run, err)
		}
		return nil
	case buildmodel.BuildStatusAborted:
		if err := run.Transition(model.StatusAborted, s.clock()); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, run); err != nil {
			return s.resolveConcurrent(ctx, run, err)
		}
		return nil
	case buildmodel.BuildStatusSuccess:
		// Continue below.
	default:
		return nil
	}
	if err := run.Transition(model.StatusDeploying, s.clock()); err != nil {
		return err
	}
	if err := s.repo.Update(ctx, run); err != nil {
		return s.resolveConcurrent(ctx, run, err)
	}
	return s.publishWithBudget(ctx, run)
}

func (s *OnBuildCompleted) resolveConcurrent(
	ctx context.Context,
	stale *model.DeploymentRun,
	updateErr error,
) error {
	if !errors.Is(updateErr, domain.ErrDeploymentRunConcurrentUpdate) {
		return updateErr
	}
	latest, err := s.repo.FindByID(ctx, stale.ID)
	if err != nil {
		return err
	}
	if latest.Status == model.StatusFailed || latest.Status == model.StatusAborted ||
		latest.Status == model.StatusSucceeded || latest.Status == model.StatusDeploying {
		return nil
	}
	return updateErr
}

func (s *OnBuildCompleted) publishWithBudget(ctx context.Context, run *model.DeploymentRun) error {
	publishErr := s.publisher.PublishRun(ctx, run)
	if publishErr == nil {
		return nil
	}
	updated, err := s.repo.RecordInfrastructureFailure(ctx, run.ID, run.Version, "deployment message publish failed")
	if errors.Is(err, domain.ErrDeploymentRunConcurrentUpdate) {
		latest, findErr := s.repo.FindByID(ctx, run.ID)
		if findErr != nil {
			return findErr
		}
		if latest.Status == model.StatusFailed || latest.Status == model.StatusAborted || latest.Status == model.StatusSucceeded {
			return nil
		}
		updated, err = s.repo.RecordInfrastructureFailure(ctx, latest.ID, latest.Version, "deployment message publish failed")
	}
	if err != nil {
		return err
	}
	if updated.Status == model.StatusFailed {
		return nil
	}
	return publishErr
}

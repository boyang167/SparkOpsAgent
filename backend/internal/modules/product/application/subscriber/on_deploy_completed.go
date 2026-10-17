package subscriber

import (
	"context"
	"time"

	deploymodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

type OnDeployCompleted struct {
	environments repository.EnvironmentRepository
}

func NewOnDeployCompleted(environments repository.EnvironmentRepository) *OnDeployCompleted {
	return &OnDeployCompleted{environments: environments}
}

func (s *OnDeployCompleted) Handle(ctx context.Context, e events.Event) error {
	ev, ok := e.(deliveryevent.DeployCompleted)
	if !ok {
		return nil
	}
	if ev.EnvironmentID == nil || ev.Status != string(deploymodel.StatusSucceeded) {
		return nil
	}
	if ev.RunType == string(deploymodel.RunTypeUndeploy) {
		row, err := s.environments.FindEnvironmentProject(ctx, *ev.EnvironmentID, ev.ProjectID)
		if err != nil {
			if err == domain.ErrEnvironmentProjectNotFound {
				return nil
			}
			return err
		}
		if row.LastDeployedAt != nil && row.LastDeployedAt.After(ev.OccurredAt()) {
			return nil
		}
		row.CurrentImageTag = ""
		row.CurrentDeploymentRunID = nil
		return s.environments.SaveEnvironmentProject(ctx, row)
	}
	row, err := s.environments.FindEnvironmentProject(ctx, *ev.EnvironmentID, ev.ProjectID)
	if err != nil {
		if err == domain.ErrEnvironmentProjectNotFound {
			return s.environments.SaveEnvironmentProject(ctx, &model.EnvironmentProject{
				EnvironmentID: *ev.EnvironmentID, ProjectID: ev.ProjectID, Enabled: true,
				CurrentImageTag: ev.ImageTag, CurrentDeploymentRunID: &ev.RunID, LastDeployedAt: ptrTime(ev.OccurredAt()),
			})
		}
		return err
	}
	if row.LastDeployedAt != nil && row.LastDeployedAt.After(ev.OccurredAt()) {
		return nil
	}
	row.CurrentImageTag = ev.ImageTag
	row.CurrentDeploymentRunID = &ev.RunID
	now := ev.OccurredAt()
	row.LastDeployedAt = &now
	return s.environments.SaveEnvironmentProject(ctx, row)
}

func ptrTime(t time.Time) *time.Time { return &t }

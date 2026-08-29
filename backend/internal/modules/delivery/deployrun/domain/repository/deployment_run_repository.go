package repository

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type DeploymentRunRepository interface {
	Create(ctx context.Context, run *model.DeploymentRun) error
	Update(ctx context.Context, run *model.DeploymentRun) error
	FindByID(ctx context.Context, id kernel.DeploymentRunID) (*model.DeploymentRun, error)
	FindByBuildID(ctx context.Context, buildID kernel.BuildID) (*model.DeploymentRun, error)
	ListByProject(ctx context.Context, projectID kernel.ProjectID, limit int) ([]*model.DeploymentRun, error)
	ListActive(ctx context.Context) ([]*model.DeploymentRun, error)
	TryAcquireLease(ctx context.Context, runID kernel.DeploymentRunID, expectedVersion int64, owner string, ttl time.Duration) (bool, error)
	ReleaseLease(ctx context.Context, runID kernel.DeploymentRunID, expectedVersion int64, owner string) error
	RecordInfrastructureFailure(ctx context.Context, runID kernel.DeploymentRunID, expectedVersion int64, message string) (*model.DeploymentRun, error)
	ListEvents(ctx context.Context, runID kernel.DeploymentRunID) ([]model.DeploymentRunEvent, error)
	HasActiveByProject(ctx context.Context, projectID kernel.ProjectID) (bool, error)
}

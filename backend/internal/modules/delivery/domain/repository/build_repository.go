package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type BuildRepository interface {
	Save(ctx context.Context, build *model.Build) error
	Update(ctx context.Context, build *model.Build) error
	FindByID(ctx context.Context, id kernel.BuildID) (*model.Build, error)
	ListByProject(ctx context.Context, projectID kernel.ProjectID, limit int) ([]*model.Build, error)
	HasRunningBuild(ctx context.Context, projectID kernel.ProjectID) (bool, error)
	NextBuildNumber(ctx context.Context, projectID kernel.ProjectID) (int, error)
}

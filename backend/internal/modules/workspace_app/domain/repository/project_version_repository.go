package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectVersionRepository interface {
	Create(ctx context.Context, version *model.ProjectVersion) error
	FindByID(ctx context.Context, projectID kernel.ProjectID, versionID kernel.ProjectVersionID) (*model.ProjectVersion, error)
	FindByVersionID(ctx context.Context, versionID kernel.ProjectVersionID) (*model.ProjectVersion, error)
	ListByProject(ctx context.Context, projectID kernel.ProjectID, limit, offset int) ([]*model.ProjectVersion, error)
}

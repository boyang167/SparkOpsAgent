package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectRepository interface {
	Save(ctx context.Context, project *model.Project) error
	Update(ctx context.Context, project *model.Project) error
	FindByID(ctx context.Context, id kernel.ProjectID) (*model.Project, error)
	FindByWorkspaceAndName(ctx context.Context, wsID kernel.WorkspaceID, name model.ProjectName) (*model.Project, error)
	ListByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Project, error)
	CountByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (int, error)
}

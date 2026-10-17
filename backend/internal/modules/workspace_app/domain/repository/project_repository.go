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
	ListByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.Project, error)
	CountByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (int, error)
	CountByProduct(ctx context.Context, productID kernel.ProductID) (int, error)
	ListByStatus(ctx context.Context, status model.ProjectStatus) ([]*model.Project, error)
}

type ProjectTerminalRepository interface {
	ProjectRepository
	BeginArchive(
		ctx context.Context,
		id kernel.ProjectID,
		generation string,
		allowUndeploy bool,
	) (bool, error)
	InitializeArchiveGeneration(ctx context.Context, id kernel.ProjectID, generation string) (bool, error)
	TransitionArchivePhase(
		ctx context.Context,
		id kernel.ProjectID,
		generation string,
		expected model.ArchivePhase,
		target model.ArchivePhase,
	) (bool, error)
	TransitionArchiving(
		ctx context.Context,
		id kernel.ProjectID,
		generation string,
		expected model.ArchivePhase,
		target model.ProjectStatus,
	) (bool, error)
}

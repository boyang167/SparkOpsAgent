package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkflowRepository interface {
	Save(ctx context.Context, workflow *model.Workflow) error
	Update(ctx context.Context, workflow *model.Workflow) error
	FindByID(ctx context.Context, id kernel.WorkflowID) (*model.Workflow, error)
	FindByProjectAndName(ctx context.Context, projectID kernel.ProjectID, name model.WorkflowName) (*model.Workflow, error)
	ListByProject(ctx context.Context, projectID kernel.ProjectID) ([]*model.Workflow, error)
}

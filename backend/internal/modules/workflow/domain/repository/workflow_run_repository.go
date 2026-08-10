package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkflowRunRepository interface {
	Save(ctx context.Context, run *model.WorkflowRun) error
	Update(ctx context.Context, run *model.WorkflowRun) error
	FindByID(ctx context.Context, id kernel.WorkflowRunID) (*model.WorkflowRun, error)
	ListByWorkflow(ctx context.Context, workflowID kernel.WorkflowID, limit int) ([]*model.WorkflowRun, error)
}

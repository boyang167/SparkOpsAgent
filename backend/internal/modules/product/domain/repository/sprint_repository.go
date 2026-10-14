package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SprintRepository interface {
	SaveSprint(ctx context.Context, sprint *model.Sprint) error
	UpdateSprint(ctx context.Context, sprint *model.Sprint) error
	FindSprintByID(ctx context.Context, id kernel.SprintID) (*model.Sprint, error)
	FindSprintByLegacyPlanID(ctx context.Context, planID kernel.ProductPlanID) (*model.Sprint, error)
	ListSprintsByProduct(ctx context.Context, productID kernel.ProductID, status string) ([]*model.Sprint, error)
	ListActiveSprintsByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Sprint, error)
	SprintNameExists(ctx context.Context, productID kernel.ProductID, name string, excludeID kernel.SprintID) (bool, error)
	// DeleteSprint soft-deletes the sprint together with its work items.
	DeleteSprint(ctx context.Context, id kernel.SprintID) error

	SaveWorkItem(ctx context.Context, item *model.WorkItem) error
	UpdateWorkItem(ctx context.Context, item *model.WorkItem) error
	DeleteWorkItem(ctx context.Context, id kernel.WorkItemID) error
	FindWorkItemByID(ctx context.Context, id kernel.WorkItemID) (*model.WorkItem, error)
	ListWorkItemsBySprint(ctx context.Context, sprintID kernel.SprintID) ([]*model.WorkItem, error)
	CountWorkItemsBySprint(ctx context.Context, sprintID kernel.SprintID) (total, done int, err error)

	SaveExecution(ctx context.Context, exec *model.WorkItemExecution) error
	UpdateExecution(ctx context.Context, exec *model.WorkItemExecution) error
	FindExecutionByWorkflowRunID(ctx context.Context, runID kernel.ProductWorkflowRunID) (*model.WorkItemExecution, error)
	ListExecutionsByWorkItem(ctx context.Context, workItemID kernel.WorkItemID) ([]*model.WorkItemExecution, error)
}

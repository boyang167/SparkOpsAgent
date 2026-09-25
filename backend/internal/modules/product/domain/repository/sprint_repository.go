package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SprintRepository interface {
	FindDefaultProcessByProduct(ctx context.Context, productID kernel.ProductID) (*model.SprintProcess, error)
	FindProcessByID(ctx context.Context, productID kernel.ProductID, processID kernel.SprintProcessID) (*model.SprintProcess, error)
	ListProcessesByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.SprintProcess, error)
	SaveProcess(ctx context.Context, process *model.SprintProcess) error
	UpdateProcess(ctx context.Context, process *model.SprintProcess) error
	DeleteProcess(ctx context.Context, productID kernel.ProductID, processID kernel.SprintProcessID) error
	CountSprintsByProcess(ctx context.Context, processID kernel.SprintProcessID) (int, error)

	ListWorkspaceSprintTemplates(ctx context.Context, workspaceID kernel.WorkspaceID) ([]*model.WorkspaceSprintProcessTemplate, error)
	SaveWorkspaceSprintTemplate(ctx context.Context, tpl *model.WorkspaceSprintProcessTemplate) error
	FindWorkspaceSprintTemplateByID(ctx context.Context, workspaceID kernel.WorkspaceID, id kernel.WorkspaceSprintProcessTemplateID) (*model.WorkspaceSprintProcessTemplate, error)
	DeleteWorkspaceSprintTemplate(ctx context.Context, workspaceID kernel.WorkspaceID, id kernel.WorkspaceSprintProcessTemplateID) error

	SaveSprint(ctx context.Context, sprint *model.Sprint) error
	UpdateSprint(ctx context.Context, sprint *model.Sprint) error
	FindSprintByID(ctx context.Context, id kernel.SprintID) (*model.Sprint, error)
	FindSprintByLegacyPlanID(ctx context.Context, planID kernel.ProductPlanID) (*model.Sprint, error)
	ListSprintsByProduct(ctx context.Context, productID kernel.ProductID, status string) ([]*model.Sprint, error)
	ListActiveSprintsByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Sprint, error)
	SprintNameExists(ctx context.Context, productID kernel.ProductID, name string, excludeID kernel.SprintID) (bool, error)

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

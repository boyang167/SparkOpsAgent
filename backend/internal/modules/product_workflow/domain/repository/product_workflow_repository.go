package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProductWorkflowRepository interface {
	SaveWorkflow(ctx context.Context, wf *model.ProductWorkflow) error
	UpdateWorkflow(ctx context.Context, wf *model.ProductWorkflow) error
	DeleteWorkflow(ctx context.Context, id kernel.ProductWorkflowID) error
	FindWorkflowByID(ctx context.Context, id kernel.ProductWorkflowID) (*model.ProductWorkflow, error)
	ListWorkflowsByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.ProductWorkflow, error)
	ListWorkflowsByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.ProductWorkflow, error)

	SaveRun(ctx context.Context, run *model.ProductWorkflowRun) error
	UpdateRun(ctx context.Context, run *model.ProductWorkflowRun) error
	FindRunByID(ctx context.Context, id kernel.ProductWorkflowRunID) (*model.ProductWorkflowRun, error)
	ListRunsByWorkflow(ctx context.Context, workflowID kernel.ProductWorkflowID, limit int) ([]*model.ProductWorkflowRun, error)

	SaveBuildRef(ctx context.Context, ref *model.WorkflowBuildRef) error
	FindRunByBuildID(ctx context.Context, buildID kernel.BuildID) (*model.WorkflowBuildRef, error)
	SaveDeployRef(ctx context.Context, ref *model.WorkflowDeployRef) error
	FindRunByDeploymentID(ctx context.Context, deploymentID kernel.DeploymentRunID) (*model.WorkflowDeployRef, error)
}

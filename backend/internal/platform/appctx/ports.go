package appctx

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type BuildTrigger interface {
	TriggerAutomated(ctx context.Context, projectID kernel.ProjectID, branch string) (kernel.BuildID, error)
}

type WorkflowDeploymentRequest struct {
	ProjectID     kernel.ProjectID
	EnvironmentID kernel.EnvironmentID
	GitRefType    string
	GitRefValue   string
	ImageTag      string
}

type DeploymentTrigger interface {
	TriggerForEnvironment(ctx context.Context, req WorkflowDeploymentRequest) (kernel.DeploymentRunID, error)
}

type WorkflowRunTrigger interface {
	StartForProduct(ctx context.Context, productID kernel.ProductID, workflowID kernel.ProductWorkflowID) (kernel.ProductWorkflowRunID, error)
}

package appctx

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type AutomatedBuildRequest struct {
	ProjectID       kernel.ProjectID
	Branch          string
	ImageRepository string
	ImageTag        string
	GitRefType      string
	Variables       map[string]string
}

type BuildTrigger interface {
	TriggerAutomated(ctx context.Context, projectID kernel.ProjectID, branch string) (kernel.BuildID, error)
	TriggerAutomatedBuild(ctx context.Context, req AutomatedBuildRequest) (kernel.BuildID, error)
}

type WorkflowDeploymentRequest struct {
	ProjectID       kernel.ProjectID
	EnvironmentID   kernel.EnvironmentID
	GitRefType      string
	GitRefValue     string
	ImageTag        string
	ImageRepository string
	DeployContents  []string
	ConfigResources []WorkflowConfigResource
}

type WorkflowConfigResource struct {
	Kind string
	Name string
	YAML string
}

type DeploymentTrigger interface {
	TriggerForEnvironment(ctx context.Context, req WorkflowDeploymentRequest) (kernel.DeploymentRunID, error)
}

type WorkflowRunTrigger interface {
	StartForProduct(ctx context.Context, productID kernel.ProductID, workflowID kernel.ProductWorkflowID) (kernel.ProductWorkflowRunID, error)
}

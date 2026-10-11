package appctx

import (
	"context"

	spmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/model"
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
	ProjectID        kernel.ProjectID
	EnvironmentID    kernel.EnvironmentID
	GitRefType       string
	GitRefValue      string
	ImageTag         string
	ImageRepository  string
	ProjectVersionID *kernel.ProjectVersionID
	DeployContents   []string
	ConfigResources  []WorkflowConfigResource
}

type VersionManifest struct {
	Kind    string
	Content string
}

type VersionImage struct {
	ServiceName string
	Image       string
	Tag         string
	BuildID     string
}

type VersionSnapshot struct {
	Manifests   []VersionManifest
	Images      []VersionImage
	GitRefType  string
	GitRefValue string
}

type CreateVersionRequest struct {
	WorkspaceID     kernel.WorkspaceID
	ProjectID       kernel.ProjectID
	ProductID       kernel.ProductID
	VersionTag      string
	WorkflowRunID   kernel.ProductWorkflowRunID
	WorkflowJobName string
	GitRefType      string
	GitRefValue     string
	ImageTag        string
	ImageRepository string
	BuildID         string
	CreatedBy       kernel.MemberID
}

type VersionCreator interface {
	CreateFromWorkflow(ctx context.Context, req CreateVersionRequest) (kernel.ProjectVersionID, error)
}

type ProjectVersionReader interface {
	LoadSnapshot(ctx context.Context, versionID kernel.ProjectVersionID) (VersionSnapshot, error)
}

type WorkflowConfigResource struct {
	Kind string
	Name string
	YAML string
}

type DeploymentTrigger interface {
	TriggerForEnvironment(ctx context.Context, req WorkflowDeploymentRequest) (kernel.DeploymentRunID, error)
}

type UndeployRequest struct {
	WorkspaceID kernel.WorkspaceID
	ProjectID   kernel.ProjectID
	MemberID    kernel.MemberID
	ClusterID   kernel.ClusterInfoID
}

// ProjectTeardown lets workspace_app take a project offline before archiving it.
type ProjectTeardown interface {
	OnlineClusters(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]kernel.ClusterInfoID, error)
	HasActiveRun(ctx context.Context, projectID kernel.ProjectID) (bool, error)
	StartUndeploy(ctx context.Context, req UndeployRequest) (kernel.DeploymentRunID, error)
	DisablePipeline(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error
}

type WorkflowRunTrigger interface {
	StartForProduct(ctx context.Context, productID kernel.ProductID, workflowID kernel.ProductWorkflowID) (kernel.ProductWorkflowRunID, error)
	StartForProductWithInput(
		ctx context.Context,
		productID kernel.ProductID,
		workflowID kernel.ProductWorkflowID,
		actor kernel.MemberID,
		input map[string]any,
	) (kernel.ProductWorkflowRunID, error)
}

type SprintProcessPort interface {
	GetOrSeedDefault(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID) (*spmodel.SprintProcess, error)
	FindByID(ctx context.Context, productID kernel.ProductID, processID kernel.SprintProcessID) (*spmodel.SprintProcess, error)
}

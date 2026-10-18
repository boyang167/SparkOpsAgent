package appctx

import (
	"context"
	"errors"

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

type EnvironmentSyncProjectOption struct {
	ProjectID         kernel.ProjectID
	SyncImage         bool
	SyncYAML          bool
	SyncResourceKinds []string
	ResourceOverrides map[string]string
	FirstSync         bool
}

type EnvironmentSyncProjectSnapshot struct {
	ProjectID kernel.ProjectID
	Option    EnvironmentSyncProjectOption
}

type EnvironmentSyncSnapshot struct {
	ID                  kernel.EnvironmentSyncSnapshotID
	WorkspaceID         kernel.WorkspaceID
	ProductID           kernel.ProductID
	WorkflowID          kernel.ProductWorkflowID
	JobName             string
	SourceEnvironmentID kernel.EnvironmentID
	TargetEnvironmentID kernel.EnvironmentID
	Projects            []EnvironmentSyncProjectSnapshot
	BoundRunID          *kernel.ProductWorkflowRunID
}

type EnvironmentSyncSnapshotBinding struct {
	SnapshotID kernel.EnvironmentSyncSnapshotID
	RunID      kernel.ProductWorkflowRunID
}

type EnvironmentSyncSnapshots interface {
	Validate(
		ctx context.Context,
		snapshotID kernel.EnvironmentSyncSnapshotID,
		productID kernel.ProductID,
		workflowID kernel.ProductWorkflowID,
		jobName string,
	) error
	BindMany(ctx context.Context, bindings []EnvironmentSyncSnapshotBinding) error
	LoadPreview(ctx context.Context, snapshotID kernel.EnvironmentSyncSnapshotID) (*EnvironmentSyncSnapshot, error)
	LoadBound(ctx context.Context, snapshotID kernel.EnvironmentSyncSnapshotID, runID kernel.ProductWorkflowRunID) (*EnvironmentSyncSnapshot, error)
}

type EnvironmentSyncTrigger interface {
	TriggerEnvironmentSync(ctx context.Context, snapshotID kernel.EnvironmentSyncSnapshotID, projectID kernel.ProjectID) (kernel.DeploymentRunID, error)
}

type UndeployRequest struct {
	WorkspaceID       kernel.WorkspaceID
	ProjectID         kernel.ProjectID
	MemberID          kernel.MemberID
	ClusterID         kernel.ClusterInfoID
	ArchiveGeneration string
}

// ProjectTeardown lets workspace_app take a project offline before archiving it.
type ProjectTeardown interface {
	OnlineClusters(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) ([]kernel.ClusterInfoID, error)
	HasActiveRun(ctx context.Context, projectID kernel.ProjectID) (bool, error)
	StartUndeploy(ctx context.Context, req UndeployRequest) (kernel.DeploymentRunID, error)
	DisablePipeline(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error
	PipelineDisabled(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) (bool, error)
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

var ErrWorkflowNotFound = errors.New("workflow not found")

type WorkflowRuntimeInputRef struct {
	StageIndex int    `json:"stage_index"`
	StageName  string `json:"stage_name"`
	JobName    string `json:"job_name"`
	JobType    string `json:"job_type"`
	Field      string `json:"field"`
}

type WorkflowRuntimeInput struct {
	Key      string                    `json:"key"`
	Label    string                    `json:"label"`
	Declared bool                      `json:"declared"`
	Secret   bool                      `json:"secret,omitempty"`
	Refs     []WorkflowRuntimeInputRef `json:"refs,omitempty"`
}

type WorkflowRuntimeInputReader interface {
	RuntimeInputs(ctx context.Context, productID kernel.ProductID, workflowID kernel.ProductWorkflowID, input map[string]any) ([]WorkflowRuntimeInput, error)
}

type SprintProcessPort interface {
	GetOrSeedDefault(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID) (*spmodel.SprintProcess, error)
	FindByID(ctx context.Context, productID kernel.ProductID, processID kernel.SprintProcessID) (*spmodel.SprintProcess, error)
}

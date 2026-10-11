package port

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectChecker interface {
	ProjectInWorkspace(ctx context.Context, projectID kernel.ProjectID, wsID kernel.WorkspaceID) (bool, error)
	ProjectIsActive(ctx context.Context, projectID kernel.ProjectID) (bool, error)
	ProjectName(ctx context.Context, projectID kernel.ProjectID) (string, error)
	ProjectNamespace(ctx context.Context, projectID kernel.ProjectID) (string, error)
	ProjectAutomationContext(ctx context.Context, projectID kernel.ProjectID) (ProjectAutomationContext, error)
}

type ProjectAutomationContext struct {
	WorkspaceID kernel.WorkspaceID
	MemberID    kernel.MemberID
}

type JenkinsCredential struct {
	ID          string
	Description string
	Username    string
	Secret      string
	Kind        string
}

type JenkinsJobSpec struct {
	Folder         string
	Job            string
	PipelineScript string
	Credentials    []JenkinsCredential
}

type BuildRequest struct {
	Namespace       string
	ProjectName     string
	Folder          string
	Job             string
	GitRefType      string
	GitRefValue     string
	ImageTag        string
	ImageRepository string
	Variables       map[string]string
	RollbackOf      int
	Timeout         time.Duration
	Credentials     []JenkinsCredential
}

type GitBranchLister interface {
	ListBranches(ctx context.Context, projectName string) ([]GitBranch, error)
}

type GitBranch struct {
	Name      string
	Default   bool
	Protected bool
}

type JenkinsExecutor interface {
	CreateOrUpdate(ctx context.Context, spec JenkinsJobSpec) error
	SyncCredentials(ctx context.Context, folder string, credentials []JenkinsCredential) error
	RunStreaming(
		ctx context.Context,
		buildID kernel.BuildID,
		req BuildRequest,
		hub LogPublisher,
	) (consoleLog string, err error)
}

type JenkinsJobDisabler interface {
	DisableJob(ctx context.Context, folder, job string) error
}

// JenkinsExecutorProvider resolves a Jenkins client from project-bound secrets.
type JenkinsExecutorProvider interface {
	ForProject(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) (JenkinsExecutor, error)
}

type LogPublisher interface {
	Publish(buildID kernel.BuildID, delta string)
}

package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectChecker interface {
	ProjectInWorkspace(ctx context.Context, projectID kernel.ProjectID, wsID kernel.WorkspaceID) (bool, error)
	ProjectIsActive(ctx context.Context, projectID kernel.ProjectID) (bool, error)
	ProjectName(ctx context.Context, projectID kernel.ProjectID) (string, error)
	ProjectAutomationContext(ctx context.Context, projectID kernel.ProjectID) (ProjectAutomationContext, error)
}

type ProjectAutomationContext struct {
	WorkspaceID kernel.WorkspaceID
	MemberID    kernel.MemberID
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
	RunStreaming(
		ctx context.Context,
		buildID kernel.BuildID,
		projectName, branch string,
		buildNumber int,
		rollbackOf int,
		hub LogPublisher,
	) (consoleLog string, err error)
}

type LogPublisher interface {
	Publish(buildID kernel.BuildID, delta string)
}

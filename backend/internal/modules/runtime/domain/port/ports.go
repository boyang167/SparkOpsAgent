package port

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectChecker interface {
	ProjectInWorkspace(ctx context.Context, projectID kernel.ProjectID, wsID kernel.WorkspaceID) (bool, error)
	ProjectName(ctx context.Context, projectID kernel.ProjectID) (string, error)
	ProjectNamespace(ctx context.Context, projectID kernel.ProjectID, projectName string) (string, error)
}

type PodLister interface {
	ListByProject(ctx context.Context, projectID kernel.ProjectID, projectName string) ([]*model.Pod, error)
	FindByID(ctx context.Context, projectID kernel.ProjectID, projectName string, podID kernel.PodID) (*model.Pod, error)
}

type PodLogReader interface {
	ReadLogs(ctx context.Context, pod *model.Pod) (string, error)
}

type PodTerminal interface {
	StreamLines(ctx context.Context, pod *model.Pod) []string
}

type Manifest struct {
	YAML string
}

type DeploymentTarget struct {
	// ClusterID and Server must come from the same workspace-authorized
	// ProjectDeploymentContext. Runtime validates the kubeconfig endpoint but
	// deliberately does not query platform_admin to reconstruct that binding.
	ClusterID      string
	Server         string
	Kubeconfig     string
	Namespace      string
	DeploymentName string
	ServiceName    string
	ImageTag       string
	RolloutTimeout time.Duration
}

type ManifestDeployer interface {
	Deploy(ctx context.Context, target DeploymentTarget, manifests []Manifest) error
}

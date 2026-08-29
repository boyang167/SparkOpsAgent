package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type DeploymentCluster struct {
	ID     kernel.ClusterInfoID
	Name   string
	Server string
}

type DeploymentSecret struct {
	ID          kernel.SecretID
	ServiceType string
	ServiceURL  string
	Username    string
	Credentials DeploymentCredentials
}

type DeploymentCredentials struct {
	Password    string
	Token       string
	APIKey      string
	PrivateKey  string
	PublicKey   string
	Certificate string
	ExtraConfig string
}

type ProjectDeploymentContext struct {
	WorkspaceID      kernel.WorkspaceID
	ProjectID        kernel.ProjectID
	Name             string
	Namespace        string
	Jenkinsfile      string
	ServiceYAML      string
	DeploymentYAML   string
	ImageRepository  string
	EnableStaticScan bool
	Cluster          DeploymentCluster
	Git              DeploymentSecret
	Harbor           DeploymentSecret
	Jenkins          DeploymentSecret
	Kubernetes       DeploymentSecret
	Sonar            *DeploymentSecret
}

type PipelineRenderRequest struct {
	WorkspaceID          kernel.WorkspaceID
	ProjectID            kernel.ProjectID
	GitRefValue          string
	ImageTag             string
	ImageRepository      string
	GitCredentialID      string
	RegistryCredentialID string
}

type PipelineRenderer interface {
	Render(ctx context.Context, request PipelineRenderRequest) (string, error)
}

type ProjectDeploymentReader interface {
	Load(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, clusterID kernel.ClusterInfoID) (ProjectDeploymentContext, error)
	ListAuthorizedClusters(ctx context.Context, wsID kernel.WorkspaceID) ([]DeploymentCluster, error)
}

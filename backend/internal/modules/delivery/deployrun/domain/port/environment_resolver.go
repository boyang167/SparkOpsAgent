package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type EnvironmentDeployTarget struct {
	EnvironmentID kernel.EnvironmentID
	ProductID     kernel.ProductID
	ClusterID     kernel.ClusterInfoID
	Namespace     string
}

type EnvironmentResolver interface {
	Resolve(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, envID kernel.EnvironmentID) (EnvironmentDeployTarget, error)
}

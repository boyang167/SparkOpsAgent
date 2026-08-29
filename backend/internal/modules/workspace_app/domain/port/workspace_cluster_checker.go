package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkspaceClusterChecker interface {
	WorkspaceClusterGranted(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (bool, error)
}

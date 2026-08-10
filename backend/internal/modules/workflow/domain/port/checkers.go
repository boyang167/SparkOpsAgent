package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectChecker interface {
	ProjectInWorkspace(ctx context.Context, projectID kernel.ProjectID, wsID kernel.WorkspaceID) (bool, error)
	ProjectIsActive(ctx context.Context, projectID kernel.ProjectID) (bool, error)
}

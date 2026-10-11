package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectLifecycle struct {
	Name   string
	Status string
}

type ProjectLifecycleReader interface {
	ProjectLifecycle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) (ProjectLifecycle, error)
}

package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectLifecycle struct {
	Name              string
	Status            string
	ProductID         string
	ArchiveGeneration string
	ArchivePhase      string
}

type ProjectLifecycleReader interface {
	ProjectLifecycle(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) (ProjectLifecycle, error)
}

type ProjectArchiveFence interface {
	ProjectLifecycleReader
	TransitionArchivePhase(
		ctx context.Context,
		projectID kernel.ProjectID,
		generation string,
		expected string,
		target string,
	) (bool, error)
}

package appctx

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type BuildTrigger interface {
	TriggerAutomated(ctx context.Context, projectID kernel.ProjectID, branch string) (kernel.BuildID, error)
}

type SyncJobCreator interface {
	CreateSyncJob(
		ctx context.Context,
		memberID kernel.MemberID,
		wsID kernel.WorkspaceID,
		name, version, sourceCluster, targetCluster, syncResources string,
	) (kernel.SyncJobID, error)
}

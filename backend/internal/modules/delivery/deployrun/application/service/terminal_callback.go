package service

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

func NewTerminalCallback(bus messaging.Bus) func(context.Context, *model.DeploymentRun) error {
	return func(ctx context.Context, run *model.DeploymentRun) error {
		if run == nil {
			return nil
		}
		completed := deliveryevent.NewDeployCompleted(
			run.ID,
			run.ProjectID,
			run.WorkspaceID,
			run.EnvironmentID,
			"",
			string(run.Status),
			string(run.RunType),
			run.ImageTag,
		)
		completed.SourceEnvironmentID = run.SourceEnvironmentID
		completed.EnvironmentSyncSnapshotID = run.EnvironmentSyncSnapshotID
		completed.CreatedBy = run.CreatedBy
		return bus.Publish(ctx, completed)
	}
}

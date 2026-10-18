package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type EnvironmentSyncSnapshotRepository interface {
	Create(ctx context.Context, snapshot *model.EnvironmentSyncSnapshot) error
	FindByID(ctx context.Context, id kernel.EnvironmentSyncSnapshotID) (*model.EnvironmentSyncSnapshot, error)
	BindToRun(ctx context.Context, id kernel.EnvironmentSyncSnapshotID, runID kernel.ProductWorkflowRunID) error
}

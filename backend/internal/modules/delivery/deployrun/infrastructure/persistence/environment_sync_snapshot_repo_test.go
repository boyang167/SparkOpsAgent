package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newEnvironmentSyncSnapshotRepo(t *testing.T) (*EnvironmentSyncSnapshotRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS delivery").Error)
	require.NoError(t, db.AutoMigrate(&environmentSyncSnapshotRow{}))
	return NewEnvironmentSyncSnapshotRepository(db), db
}

func sampleEnvironmentSyncSnapshot() *model.EnvironmentSyncSnapshot {
	now := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	return &model.EnvironmentSyncSnapshot{
		ID:                  kernel.NewEnvironmentSyncSnapshotID(),
		WorkspaceID:         kernel.NewWorkspaceID(),
		ProductID:           kernel.NewProductID(),
		WorkflowID:          kernel.NewProductWorkflowID(),
		JobName:             "promote",
		SourceEnvironmentID: kernel.NewEnvironmentID(),
		TargetEnvironmentID: kernel.NewEnvironmentID(),
		Projects: []model.EnvironmentSyncProjectSnapshot{
			{
				ProjectID: kernel.NewProjectID(),
				Option: model.EnvironmentSyncProjectOption{
					ProjectID: kernel.NewProjectID(),
					SyncImage: true,
					SyncYAML:  false,
				},
				SourceResources: []model.EnvironmentSyncResource{{
					Kind: model.SyncManifestKindDeployment, Name: "demo", YAML: "kind: Deployment",
				}},
				Plan: model.EnvironmentSyncPlan{
					Diffs: []model.EnvironmentSyncDiff{{Kind: model.SyncManifestKindDeployment, Name: "demo", Action: "update"}},
				},
			},
			{
				ProjectID: kernel.NewProjectID(),
				Option: model.EnvironmentSyncProjectOption{
					SyncImage: true,
					SyncYAML:  true,
					FirstSync: true,
				},
				Plan: model.EnvironmentSyncPlan{
					ImageRepository: "registry.example/demo",
					ImageTag:        "v2",
				},
			},
		},
		Checksum:  "abc123",
		CreatedBy: kernel.NewMemberID(),
		CreatedAt: now,
	}
}

func TestEnvironmentSyncSnapshotRepositoryRoundTripAndSingleBind(t *testing.T) {
	repo, _ := newEnvironmentSyncSnapshotRepo(t)
	ctx := context.Background()
	snapshot := sampleEnvironmentSyncSnapshot()
	snapshot.Projects[0].Option.ProjectID = snapshot.Projects[0].ProjectID

	require.NoError(t, repo.Create(ctx, snapshot))

	loaded, err := repo.FindByID(ctx, snapshot.ID)
	require.NoError(t, err)
	require.Equal(t, snapshot.ID, loaded.ID)
	require.Equal(t, snapshot.WorkflowID, loaded.WorkflowID)
	require.Equal(t, snapshot.JobName, loaded.JobName)
	require.Len(t, loaded.Projects, 2)
	require.Equal(t, snapshot.Projects[0].Plan.Diffs[0].Action, loaded.Projects[0].Plan.Diffs[0].Action)
	require.Equal(t, "v2", loaded.Projects[1].Plan.ImageTag)

	runID := kernel.NewProductWorkflowRunID()
	require.NoError(t, repo.BindToRun(ctx, snapshot.ID, runID))

	rebound, err := repo.FindByID(ctx, snapshot.ID)
	require.NoError(t, err)
	require.NotNil(t, rebound.BoundRunID)
	require.Equal(t, runID, *rebound.BoundRunID)
	require.NotNil(t, rebound.BoundAt)

	err = repo.BindToRun(ctx, snapshot.ID, kernel.NewProductWorkflowRunID())
	require.ErrorIs(t, err, domain.ErrEnvironmentSyncSnapshotAlreadyBound)
}

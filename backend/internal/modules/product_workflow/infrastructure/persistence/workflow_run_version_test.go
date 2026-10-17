package persistence

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestWorkflowRunModelAutoMigrationAddsNonNullVersionDefault(t *testing.T) {
	db := openWorkflowRunVersionDB(t)

	var columns []struct {
		Name         string  `gorm:"column:name"`
		NotNull      int     `gorm:"column:notnull"`
		DefaultValue *string `gorm:"column:dflt_value"`
	}
	require.NoError(t, db.Raw("PRAGMA product.table_info('product_workflow_runs')").Scan(&columns).Error)
	for _, column := range columns {
		if column.Name != "version" {
			continue
		}
		require.Equal(t, 1, column.NotNull)
		require.NotNil(t, column.DefaultValue)
		require.Equal(t, "0", *column.DefaultValue)
		return
	}
	t.Fatal("version column not found")
}

func TestUpdateRunUsesVersionCASAndRejectsStaleWriter(t *testing.T) {
	db := openWorkflowRunVersionDB(t)
	repoA := NewProductWorkflowRepository(db)
	repoB := NewProductWorkflowRepository(db)
	run := model.NewProductWorkflowRun(
		kernel.NewProductWorkflowID(),
		kernel.NewProductID(),
		[]model.WorkflowStage{{
			Name: "build",
			Jobs: []model.WorkflowJob{{Name: "build", Type: model.JobTypeSparkopsBuild}},
		}},
	)
	require.NoError(t, repoA.SaveRun(context.Background(), run))

	first, err := repoA.FindRunByID(context.Background(), run.ID)
	require.NoError(t, err)
	stale, err := repoB.FindRunByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Zero(t, first.Version)
	require.Zero(t, stale.Version)

	first.StageRuns[0].Status = model.JobRunStatusSuccess
	require.NoError(t, repoA.UpdateRun(context.Background(), first))
	require.Equal(t, int64(1), first.Version)

	stale.StageRuns[0].Status = model.JobRunStatusFailed
	err = repoB.UpdateRun(context.Background(), stale)
	require.ErrorIs(t, err, domain.ErrProductWorkflowRunConflict)

	found, err := repoA.FindRunByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), found.Version)
	require.Equal(t, model.JobRunStatusSuccess, found.StageRuns[0].Status)
}

func TestIndependentRepositoriesReloadAndMergeDifferentStageRuns(t *testing.T) {
	db := openWorkflowRunVersionDB(t)
	repoA := NewProductWorkflowRepository(db)
	repoB := NewProductWorkflowRepository(db)
	run := model.NewProductWorkflowRun(
		kernel.NewProductWorkflowID(),
		kernel.NewProductID(),
		[]model.WorkflowStage{
			{Name: "build", Jobs: []model.WorkflowJob{{Name: "build", Type: model.JobTypeSparkopsBuild}}},
			{Name: "deploy", Jobs: []model.WorkflowJob{{Name: "deploy", Type: model.JobTypeSparkopsDeploy}}},
		},
	)
	run.Status = model.RunStatusRunning
	require.NoError(t, repoA.SaveRun(context.Background(), run))

	buildWriter, err := repoA.FindRunByID(context.Background(), run.ID)
	require.NoError(t, err)
	deployWriter, err := repoB.FindRunByID(context.Background(), run.ID)
	require.NoError(t, err)

	buildWriter.StageRuns[0].Status = model.JobRunStatusSuccess
	require.NoError(t, repoA.UpdateRun(context.Background(), buildWriter))

	deployWriter.StageRuns[1].Status = model.JobRunStatusSuccess
	require.ErrorIs(t, repoB.UpdateRun(context.Background(), deployWriter), domain.ErrProductWorkflowRunConflict)

	deployWriter, err = repoB.FindRunByID(context.Background(), run.ID)
	require.NoError(t, err)
	deployWriter.StageRuns[1].Status = model.JobRunStatusSuccess
	deployWriter.Status = model.RunStatusSuccess
	require.NoError(t, repoB.UpdateRun(context.Background(), deployWriter))

	found, err := repoA.FindRunByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, model.JobRunStatusSuccess, found.StageRuns[0].Status)
	require.Equal(t, model.JobRunStatusSuccess, found.StageRuns[1].Status)
	require.Equal(t, model.RunStatusSuccess, found.Status)
	require.Equal(t, int64(2), found.Version)
}

func openWorkflowRunVersionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS product").Error)
	require.NoError(t, db.AutoMigrate(&productWorkflowRunModel{}))
	return db
}

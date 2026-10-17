package persistence

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestProductWorkflowRunRepositoryUsesContextTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(2)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS product").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE product.product_workflow_runs (
			id TEXT PRIMARY KEY, workflow_id TEXT, product_id TEXT, status TEXT,
			trigger TEXT, input TEXT, stage_runs TEXT, current_stage INTEGER,
			error_message TEXT, started_at DATETIME, completed_at DATETIME, created_at DATETIME,
			version INTEGER NOT NULL DEFAULT 0
		)
	`).Error)

	repo := NewProductWorkflowRepository(db)
	runID := kernel.NewProductWorkflowRunID()
	workflowID := kernel.NewProductWorkflowID()
	productID := kernel.NewProductID()
	rollback := errors.New("force rollback")
	err = postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		require.NoError(t, postgres.DB(ctx, db).Exec(`
			INSERT INTO product.product_workflow_runs
				(id, workflow_id, product_id, status, trigger, input, stage_runs, current_stage, created_at)
			VALUES (?, ?, ?, 'running', '{}', '{}', '[]', 0, CURRENT_TIMESTAMP)
		`, runID.String(), workflowID.String(), productID.String()).Error)
		run, findErr := repo.FindRunByID(ctx, runID)
		require.NoError(t, findErr)
		run.Status = model.RunStatusSuccess
		require.NoError(t, repo.UpdateRun(ctx, run))
		return rollback
	})
	require.ErrorIs(t, err, rollback)

	var count int64
	require.NoError(t, db.Table("product.product_workflow_runs").Count(&count).Error)
	require.Zero(t, count)
}

package engine

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	pwfevent "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	productpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestProductWorkflowTerminalRunAndOutboxAreAtomic(t *testing.T) {
	db := openProductWorkflowAtomicityDB(t)
	repo := productpersistence.NewProductWorkflowRepository(db)
	workflow := model.NewProductWorkflow(
		kernel.NewProductID(), kernel.NewWorkspaceID(), "release", "release", nil, kernel.NewMemberID(),
	)
	require.NoError(t, repo.SaveWorkflow(context.Background(), workflow))
	run := model.NewProductWorkflowRun(workflow.ID, workflow.ProductID, nil)
	require.NoError(t, repo.SaveRun(context.Background(), run))
	now := time.Now().UTC()
	run.Status = model.RunStatusSuccess
	run.CompletedAt = &now

	scheduler := NewScheduler(
		repo, nil, nil, nil,
		messaging.NewReliableBus(
			sharedevents.NewRegistry(),
			messagingpersistence.NewOutboxStore(db),
			nil, nil, nil,
		),
		postgres.NewTransactor(db),
	)
	require.Error(t, scheduler.persistTerminalRun(context.Background(), run))
	stored, err := repo.FindRunByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, model.RunStatusPending, stored.Status)
	requireProductWorkflowAtomicityCount(t, db, "platform.event_outbox", 0)

	registry := sharedevents.NewRegistry()
	require.NoError(t, pwfevent.Register(registry))
	scheduler = NewScheduler(
		repo, nil, nil, nil,
		messaging.NewReliableBus(
			registry,
			messagingpersistence.NewOutboxStore(db),
			nil, nil, nil,
		),
		postgres.NewTransactor(db),
	)
	require.NoError(t, scheduler.persistTerminalRun(context.Background(), run))
	stored, err = repo.FindRunByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, model.RunStatusSuccess, stored.Status)
	requireProductWorkflowAtomicityCount(t, db, "platform.event_outbox", 1)
}

func openProductWorkflowAtomicityDB(t *testing.T) *gorm.DB {
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
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE product.product_workflows (
			id TEXT PRIMARY KEY, product_id TEXT, workspace_id TEXT, name TEXT, "key" TEXT,
			description TEXT, status TEXT, stages TEXT, template_binding TEXT, origin TEXT,
			created_by TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
		CREATE TABLE product.product_workflow_runs (
			id TEXT PRIMARY KEY, workflow_id TEXT, product_id TEXT, status TEXT, trigger TEXT,
			input TEXT, stage_runs TEXT, current_stage INTEGER, error_message TEXT,
			started_at DATETIME, completed_at DATETIME, created_at DATETIME,
			version INTEGER NOT NULL DEFAULT 0
		)
	`).Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	return db
}

func requireProductWorkflowAtomicityCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Count(&count).Error)
	require.Equal(t, want, count)
}

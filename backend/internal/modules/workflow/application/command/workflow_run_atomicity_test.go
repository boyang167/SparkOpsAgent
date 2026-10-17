package command_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/application/command"
	workflowevent "github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/model"
	workflowpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type atomicWorkflowProjectChecker struct{}

func (atomicWorkflowProjectChecker) ProjectInWorkspace(context.Context, kernel.ProjectID, kernel.WorkspaceID) (bool, error) {
	return true, nil
}
func (atomicWorkflowProjectChecker) ProjectIsActive(context.Context, kernel.ProjectID) (bool, error) {
	return true, nil
}

func TestWorkflowRunAndCompletionOutboxAreAtomic(t *testing.T) {
	db := openWorkflowAtomicityDB(t)
	workflows := workflowpersistence.NewWorkflowRepository(db)
	runs := workflowpersistence.NewWorkflowRunRepository(db)
	name, err := model.NewWorkflowName("release")
	require.NoError(t, err)
	workflow := model.NewWorkflow(kernel.NewProjectID(), name, "", "{}")
	require.NoError(t, workflow.Activate())
	require.NoError(t, workflows.Save(context.Background(), workflow))

	handler := command.NewStartWorkflowRunHandler(
		workflows,
		runs,
		atomicWorkflowProjectChecker{},
		messaging.NewReliableBus(
			sharedevents.NewRegistry(),
			messagingpersistence.NewOutboxStore(db),
			nil, nil, nil,
		),
		postgres.NewTransactor(db),
	)
	_, err = handler.Handle(
		context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), workflow.ID, "",
	)
	require.Error(t, err)
	requireWorkflowAtomicityCount(t, db, "workflow.workflow_runs", 0)
	requireWorkflowAtomicityCount(t, db, "platform.event_outbox", 0)

	registry := sharedevents.NewRegistry()
	require.NoError(t, workflowevent.Register(registry))
	handler = command.NewStartWorkflowRunHandler(
		workflows,
		runs,
		atomicWorkflowProjectChecker{},
		messaging.NewReliableBus(
			registry,
			messagingpersistence.NewOutboxStore(db),
			nil, nil, nil,
		),
		postgres.NewTransactor(db),
	)
	_, err = handler.Handle(
		context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), workflow.ID, "",
	)
	require.NoError(t, err)
	requireWorkflowAtomicityCount(t, db, "workflow.workflow_runs", 1)
	requireWorkflowAtomicityCount(t, db, "platform.event_outbox", 1)
}

func openWorkflowAtomicityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS workflow").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE workflow.workflows (
			id TEXT PRIMARY KEY, project_id TEXT, name TEXT, description TEXT,
			status TEXT, definition TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
		CREATE TABLE workflow.workflow_runs (
			id TEXT PRIMARY KEY, workflow_id TEXT, member_id TEXT, workspace_id TEXT,
			status TEXT, input TEXT, output TEXT, error TEXT, started_at DATETIME,
			completed_at DATETIME, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		)
	`).Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	return db
}

func requireWorkflowAtomicityCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Count(&count).Error)
	require.Equal(t, want, count)
}

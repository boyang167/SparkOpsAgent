package command

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	deliverypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestBuildTerminalUpdateAndOutboxAreAtomic(t *testing.T) {
	t.Run("outbox failure rolls back terminal update", func(t *testing.T) {
		db := openBuildAtomicityDB(t)
		repo := deliverypersistence.NewBuildRepository(db)
		build := newStoredRunningBuild(t, repo)
		build.Succeed(1, "orders", "ok")

		handler := &TriggerBuildHandler{
			builds: repo,
			bus: messaging.NewReliableBus(
				sharedevents.NewRegistry(),
				messagingpersistence.NewOutboxStore(db),
				nil, nil, nil,
			),
			tx: postgres.NewTransactor(db),
		}
		err := handler.persistTerminalBuild(
			context.Background(), build, "orders", build.MemberID, build.WorkspaceID, build.ProjectID,
		)
		require.Error(t, err)

		stored, findErr := repo.FindByID(context.Background(), build.ID)
		require.NoError(t, findErr)
		require.Equal(t, model.BuildStatusRunning, stored.Status)
		requireBuildAtomicityCount(t, db, "platform.event_outbox", 0)
	})

	t.Run("success commits terminal update and outbox", func(t *testing.T) {
		db := openBuildAtomicityDB(t)
		repo := deliverypersistence.NewBuildRepository(db)
		build := newStoredRunningBuild(t, repo)
		build.Succeed(1, "orders", "ok")
		registry := sharedevents.NewRegistry()
		require.NoError(t, deliveryevent.Register(registry))

		handler := &TriggerBuildHandler{
			builds: repo,
			bus: messaging.NewReliableBus(
				registry,
				messagingpersistence.NewOutboxStore(db),
				nil, nil, nil,
			),
			tx: postgres.NewTransactor(db),
		}
		require.NoError(t, handler.persistTerminalBuild(
			context.Background(), build, "orders", build.MemberID, build.WorkspaceID, build.ProjectID,
		))

		stored, err := repo.FindByID(context.Background(), build.ID)
		require.NoError(t, err)
		require.Equal(t, model.BuildStatusSuccess, stored.Status)
		requireBuildAtomicityCount(t, db, "platform.event_outbox", 1)
	})
}

func openBuildAtomicityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS delivery").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE delivery.builds (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			member_id TEXT,
			workspace_id TEXT,
			status TEXT,
			branch TEXT,
			build_number INTEGER,
			jenkins_job TEXT,
			console_log TEXT,
			error TEXT,
			rollback_of_build_id TEXT,
			started_at DATETIME,
			execution_started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)
	`).Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	return db
}

func newStoredRunningBuild(t *testing.T, repo interface {
	Save(context.Context, *model.Build) error
}) *model.Build {
	t.Helper()
	build := model.NewBuild(kernel.NewProjectID(), kernel.NewMemberID(), kernel.NewWorkspaceID(), "main")
	require.NoError(t, repo.Save(context.Background(), build))
	return build
}

func requireBuildAtomicityCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Count(&count).Error)
	require.Equal(t, want, count)
}

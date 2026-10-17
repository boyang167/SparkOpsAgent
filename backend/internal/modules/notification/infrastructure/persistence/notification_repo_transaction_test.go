package persistence

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestNotificationRepositoryUsesContextTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(2)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS notification").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE notification.notifications (
			id TEXT PRIMARY KEY,
			member_id TEXT,
			workspace_id TEXT,
			type TEXT,
			title TEXT,
			body TEXT,
			link TEXT,
			read_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)
	`).Error)

	repo := NewNotificationRepository(db)
	rollback := errors.New("force rollback")
	err = postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		notification, createErr := model.NewNotification(
			kernel.NewMemberID(),
			kernel.NewWorkspaceID(),
			model.NotificationTypeBuildCompleted,
			"build complete",
			"success",
			"/builds/1",
		)
		require.NoError(t, createErr)
		require.NoError(t, repo.Save(ctx, notification))
		return rollback
	})
	require.ErrorIs(t, err, rollback)

	var count int64
	require.NoError(t, db.Table("notification.notifications").Count(&count).Error)
	require.Zero(t, count, "notification must roll back with the Inbox transaction")
}

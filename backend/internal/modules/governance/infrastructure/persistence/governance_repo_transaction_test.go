package persistence

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestGovernanceStoreUsesContextTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(2)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS governance").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE governance.roles (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			parent_id TEXT,
			name TEXT,
			permissions BLOB,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		);
		CREATE TABLE governance.menus (
			id TEXT PRIMARY KEY,
			workspace_id TEXT,
			parent_id TEXT,
			name TEXT,
			path TEXT,
			title TEXT,
			icon TEXT,
			component TEXT,
			hidden BOOLEAN,
			sort INTEGER,
			keep_alive BOOLEAN,
			active_menu TEXT,
			close_tab BOOLEAN,
			is_basic BOOLEAN,
			transition_animation TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)
	`).Error)

	store := NewStore(db)
	menus := NewMenuStore(db)
	rollback := errors.New("force rollback")
	err = postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		workspaceID := kernel.NewWorkspaceID()
		require.NoError(t, store.SaveRole(ctx, model.NewRole(
			workspaceID,
			"admin",
			[]string{"governance.admin"},
			nil,
		)))
		require.NoError(t, menus.Save(ctx, &model.Menu{
			WorkspaceID: workspaceID,
			Name:        "projects",
			Path:        "/projects",
			Title:       "Projects",
		}))
		return rollback
	})
	require.ErrorIs(t, err, rollback)

	var count int64
	require.NoError(t, db.Table("governance.roles").Count(&count).Error)
	require.Zero(t, count, "roles must roll back with the Inbox transaction")
	require.NoError(t, db.Table("governance.menus").Count(&count).Error)
	require.Zero(t, count, "menus must roll back with the Inbox transaction")
}

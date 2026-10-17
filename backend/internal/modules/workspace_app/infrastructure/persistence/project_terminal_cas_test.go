package persistence

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestProjectTerminalCASTwoTransactionsCannotOverwriteArchiveWinner(t *testing.T) {
	repo, tx, projectID, generation := openProjectTerminalCASDB(t)

	var archived bool
	require.NoError(t, tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		var err error
		archived, err = repo.TransitionArchiving(ctx, projectID, generation, model.ProjectStatusArchived)
		return err
	}))
	var activated bool
	require.NoError(t, tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		var err error
		activated, err = repo.TransitionArchiving(ctx, projectID, generation, model.ProjectStatusActive)
		return err
	}))

	require.True(t, archived)
	require.False(t, activated)
	require.Equal(t, model.ProjectStatusArchived, readProjectStatus(t, repo.db, projectID))
}

func TestProjectTerminalCASTwoTransactionsCannotOverwriteAbortWinner(t *testing.T) {
	repo, tx, projectID, generation := openProjectTerminalCASDB(t)

	var activated bool
	require.NoError(t, tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		var err error
		activated, err = repo.TransitionArchiving(ctx, projectID, generation, model.ProjectStatusActive)
		return err
	}))
	var archived bool
	require.NoError(t, tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		var err error
		archived, err = repo.TransitionArchiving(ctx, projectID, generation, model.ProjectStatusArchived)
		return err
	}))

	require.True(t, activated)
	require.False(t, archived)
	require.Equal(t, model.ProjectStatusActive, readProjectStatus(t, repo.db, projectID))
}

func TestProjectModelGORMFallbackIncludesArchiveGeneration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.Table("projects").AutoMigrate(&projectModel{}))
	var columns []struct{ Name string }
	require.NoError(t, db.Raw("PRAGMA table_info(projects)").Scan(&columns).Error)
	require.Contains(t, columns, struct{ Name string }{Name: "archive_generation"})
}

func openProjectTerminalCASDB(t *testing.T) (*ProjectRepository, postgres.Transactor, kernel.ProjectID, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS workspace_app").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE workspace_app.projects (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			archive_generation TEXT,
			updated_at DATETIME,
			deleted_at DATETIME
		)
	`).Error)
	projectID := kernel.NewProjectID()
	generation := "generation-a"
	require.NoError(t, db.Exec(
		"INSERT INTO workspace_app.projects (id, status, archive_generation) VALUES (?, ?, ?)",
		projectID.String(), model.ProjectStatusArchiving, generation,
	).Error)
	return &ProjectRepository{db: db}, postgres.NewTransactor(db), projectID, generation
}

func readProjectStatus(t *testing.T, db *gorm.DB, projectID kernel.ProjectID) model.ProjectStatus {
	t.Helper()
	var status string
	require.NoError(t, db.Table("workspace_app.projects").
		Where("id = ?", projectID.String()).Pluck("status", &status).Error)
	return model.ProjectStatus(status)
}

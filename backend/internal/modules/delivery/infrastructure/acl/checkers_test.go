package acl

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestProjectNamespaceReadsPersistedWorkspaceProjectNamespace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS workspace_app").Error)
	require.NoError(t, db.Exec(`CREATE TABLE workspace_app.projects (id TEXT PRIMARY KEY, namespace TEXT)`).Error)
	projectID := kernel.NewProjectID()
	require.NoError(t, db.Exec(
		`INSERT INTO workspace_app.projects(id, namespace) VALUES (?, 'authoritative-team')`,
		projectID.String(),
	).Error)

	namespace, err := NewWorkspaceAppACL(db).ProjectNamespace(context.Background(), projectID)

	require.NoError(t, err)
	require.Equal(t, "authoritative-team", namespace)
}

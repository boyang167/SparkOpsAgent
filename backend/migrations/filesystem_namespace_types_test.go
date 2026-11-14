package migrations

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestFilesystemNamespaceTypesMigrationDefinesSchema(t *testing.T) {
	up := readMigration(t, "000083_filesystem_namespace_types.up.sql")
	down := readMigration(t, "000083_filesystem_namespace_types.down.sql")
	for _, fragment := range []string{
		"ADD COLUMN namespace_type",
		"ADD COLUMN owner_user_id",
		"ADD COLUMN policy_version",
		"UPDATE filesystem.namespaces",
		"DROP CONSTRAINT IF EXISTS namespaces_workspace_id_key",
		"CREATE UNIQUE INDEX",
		"CHECK (namespace_type IN ('ORGANIZATION','WORKSPACE','USER'))",
	} {
		require.Contains(t, up, fragment)
	}
	require.Contains(t, down, "DROP COLUMN IF EXISTS policy_version")
}

func TestFilesystemNamespaceTypesMigrationBackfillsWorkspaceRows(t *testing.T) {
	db := openMigrationPostgres(t)
	require.NoError(t, db.Exec(readMigration(t, "000077_filesystem_kernel.up.sql")).Error)
	require.NoError(t, db.Exec(readMigration(t, "000078_filesystem_transfer.up.sql")).Error)
	require.NoError(t, db.Exec(readMigration(t, "000079_filesystem_storage_fabric.up.sql")).Error)
	require.NoError(t, db.Exec(readMigration(t, "000080_knowledge_filesystem_expand.up.sql")).Error)
	require.NoError(t, db.Exec(readMigration(t, "000081_filesystem_transfer_list.up.sql")).Error)
	require.NoError(t, db.Exec(readMigration(t, "000082_filesystem_blob_lifecycle.up.sql")).Error)

	workspaceID := uuid.New()
	require.NoError(t, db.Exec(`
		INSERT INTO filesystem.namespaces (id, organization_id, workspace_id, status)
		VALUES (?, ?, ?, 'ACTIVE')
	`, uuid.New(), uuid.New(), workspaceID).Error)
	require.NoError(t, db.Exec(readMigration(t, "000083_filesystem_namespace_types.up.sql")).Error)

	var got struct {
		NamespaceType string
		WorkspaceID   *uuid.UUID
		OwnerUserID   *uuid.UUID
		PolicyVersion int64
	}
	require.NoError(t, db.Raw(`
		SELECT namespace_type, workspace_id, owner_user_id, policy_version
		FROM filesystem.namespaces WHERE workspace_id = ?
	`, workspaceID).Scan(&got).Error)
	require.Equal(t, "WORKSPACE", got.NamespaceType)
	require.Equal(t, workspaceID, *got.WorkspaceID)
	require.Nil(t, got.OwnerUserID)
	require.EqualValues(t, 1, got.PolicyVersion)
}

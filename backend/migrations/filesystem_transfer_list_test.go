package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemTransferListMigrationAddsStableWorkspaceIndexes(t *testing.T) {
	up := readMigration(t, "000081_filesystem_transfer_list.up.sql")
	down := readMigration(t, "000081_filesystem_transfer_list.down.sql")

	require.Contains(t, up,
		"ON filesystem.transfer_sessions(workspace_id, created_at DESC, id DESC)")
	require.Contains(t, up,
		"ON filesystem.transfer_sessions(workspace_id, status, created_at DESC, id DESC)")
	require.Contains(t, down, "DROP INDEX IF EXISTS filesystem.ix_transfer_sessions_workspace_created")
	require.Contains(t, down, "DROP INDEX IF EXISTS filesystem.ix_transfer_sessions_workspace_status_created")
	requireInOrder(t, up,
		"CREATE INDEX ix_transfer_sessions_workspace_created",
		"CREATE INDEX ix_transfer_sessions_workspace_status_created",
	)
	requireInOrder(t, down,
		"DROP INDEX IF EXISTS filesystem.ix_transfer_sessions_workspace_status_created",
		"DROP INDEX IF EXISTS filesystem.ix_transfer_sessions_workspace_created",
	)
}

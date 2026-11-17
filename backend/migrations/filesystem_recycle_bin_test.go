package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemRecycleBinMigrationRedefinesActiveNameIndexes(t *testing.T) {
	up := readMigration(t, "000089_filesystem_recycle_bin.up.sql")
	down := readMigration(t, "000089_filesystem_recycle_bin.down.sql")

	require.Contains(t, up, "DROP INDEX IF EXISTS filesystem.ux_filesystem_nodes_child_name")
	require.Contains(t, up, "WHERE parent_id IS NOT NULL AND state = 'ACTIVE'")
	require.Contains(t, up, "WHERE parent_id IS NULL AND state = 'ACTIVE'")
	require.Contains(t, up, "ADD COLUMN IF NOT EXISTS deleted_by UUID")
	require.Contains(t, up, "ix_filesystem_nodes_trash_entries")

	require.Contains(t, down, "DROP INDEX IF EXISTS filesystem.ux_filesystem_nodes_child_name")
	require.Contains(t, down, "DROP COLUMN IF EXISTS deleted_by")
	require.NotContains(t, strings.ToLower(down), "state = 'active'")
}
package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemShareRequestMigrationDefinesGovernedApprovalState(t *testing.T) {
	up := readMigration(t, "000085_filesystem_share_requests.up.sql")
	down := readMigration(t, "000085_filesystem_share_requests.down.sql")

	for _, column := range []string{
		"organization_id", "namespace_id", "node_id", "requested_by",
		"principal_type", "principal_ids", "actions", "inherit_to_children",
		"classification", "status", "reviewed_by", "reviewed_at",
		"rejection_reason", "created_at",
	} {
		require.Contains(t, up, column)
	}
	require.Contains(t, up, "PENDING")
	require.Contains(t, up, "APPROVED")
	require.Contains(t, up, "REJECTED")
	require.Contains(t, up, "CANCELLED")
	require.Contains(t, up, "DOWNLOAD")
	require.Contains(t, up, "ALTER TABLE filesystem.nodes")
	require.Contains(t, up, "classification")
	require.Contains(t, up, "UPDATE filesystem.nodes")
	require.Contains(t, up, "node_type = 'DIRECTORY'")
	require.Contains(t, up, "SET classification = 'RESTRICTED'")
	require.Contains(t, up, "status = 'COMPLETED'")
	require.Contains(t, up, "MAX(CASE classification")
	require.Contains(t, up, "ALTER TABLE filesystem.export_tasks")
	require.Contains(t, up, "ALTER TABLE filesystem.transfer_sessions")
	require.Contains(t, up, "CREATE INDEX")
	require.Contains(t, down, "DELETE FROM filesystem.resource_grants")
	require.Contains(t, down, "permission = 'DOWNLOAD'")
	require.Contains(t, down, "DROP COLUMN IF EXISTS classification")
	require.Contains(t, down, "rollback data loss")
	require.Contains(t, down, "DROP TABLE")
	executable := strings.TrimSpace(down)
	for strings.HasPrefix(executable, "--") {
		_, executable, _ = strings.Cut(executable, "\n")
		executable = strings.TrimSpace(executable)
	}
	require.True(t, strings.HasPrefix(executable, "DO $"), "destructive rollback guard must be the first executable statement")
	require.Contains(t, down, "current_setting('sparkops.allow_destructive_filesystem_000085_down', true)")
	require.Contains(t, down, "RAISE EXCEPTION")
	requireInOrder(
		t, down,
		"current_setting('sparkops.allow_destructive_filesystem_000085_down', true)",
		"DELETE FROM filesystem.resource_grants",
		"DROP TABLE",
	)
}

package migrations

import (
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
	require.Contains(t, up, "CREATE INDEX")
	require.Contains(t, down, "DROP TABLE")
}

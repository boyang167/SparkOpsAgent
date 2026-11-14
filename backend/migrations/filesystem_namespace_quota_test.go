package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemNamespaceQuotaMigrationDefinesCompatibleUpAndDown(t *testing.T) {
	up := readMigration(t, "000086_filesystem_namespace_quota.up.sql")
	down := readMigration(t, "000086_filesystem_namespace_quota.down.sql")

	for _, fragment := range []string{
		"quota_accounts",
		"quota_reservations",
		"namespace_id",
		"ALTER COLUMN workspace_id DROP NOT NULL",
		"REFERENCES filesystem.namespaces(id)",
		"UNIQUE (namespace_id)",
	} {
		require.Contains(t, up, fragment)
	}
	require.Contains(t, down, "DELETE FROM filesystem.quota_accounts")
	require.Contains(t, down, "namespace_type <> 'WORKSPACE'")
	require.Contains(t, down, "ALTER COLUMN workspace_id SET NOT NULL")
	require.NotContains(t, down, "DROP TABLE filesystem.quota_accounts")
}

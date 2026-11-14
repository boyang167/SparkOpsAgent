package migrations

import (
	"strings"
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
	require.Contains(t, down, "ON filesystem.quota_reservations(workspace_id, session_id)")
	require.NotContains(t, down, "DROP TABLE filesystem.quota_accounts")
}

func TestFilesystemNamespaceQuotaMigrationDropsWorkspaceNotNullBeforeScopedInserts(t *testing.T) {
	up := strings.Join(strings.Fields(readMigration(t, "000086_filesystem_namespace_quota.up.sql")), " ")

	insertAccounts := strings.Index(up, "INSERT INTO filesystem.quota_accounts")
	accountAlter := strings.Index(up, "ALTER TABLE filesystem.quota_accounts DROP CONSTRAINT")
	require.NotEqual(t, -1, accountAlter)
	require.Less(t, accountAlter, insertAccounts)
	require.Contains(t, up[accountAlter:insertAccounts], "ALTER COLUMN workspace_id DROP NOT NULL")
	require.Contains(t, up, "ALTER TABLE filesystem.transfer_sessions DROP CONSTRAINT IF EXISTS fk_transfer_sessions_namespace_workspace, ALTER COLUMN workspace_id DROP NOT NULL")
	require.Contains(t, up, "ALTER TABLE filesystem.quota_reservations DROP CONSTRAINT IF EXISTS fk_quota_reservations_account_workspace")
	require.Contains(t, up, "ALTER COLUMN workspace_id DROP NOT NULL")
	require.Contains(t, up, "UNIQUE (namespace_id, idempotency_key)")
	require.Contains(t, up, "FOREIGN KEY (session_id) REFERENCES filesystem.transfer_sessions(id)")
}

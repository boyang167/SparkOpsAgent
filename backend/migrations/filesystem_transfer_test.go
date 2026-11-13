package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemTransferMigrationDefinesTransferSchema(t *testing.T) {
	up := readMigration(t, "000078_filesystem_transfer.up.sql")
	for _, table := range []string{"transfer_sessions", "transfer_parts"} {
		require.Contains(t, up, "CREATE TABLE filesystem."+table)
	}
	for _, invariant := range []string{
		"UNIQUE (workspace_id, idempotency_key)",
		"FOREIGN KEY (target_parent_id, namespace_id)",
		"REFERENCES filesystem.nodes(id, namespace_id)",
		"CHECK (expected_size >= 0 AND expected_size <= 21474836480)",
		"CHECK (protocol IN ('DIRECT','MULTIPART'))",
		"CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED'))",
		"CHECK (status IN ('INITIATED','UPLOADING','PAUSED','VERIFYING','SCANNING','COMMITTING','COMPLETED','FAILED','REJECTED','ABORTED','EXPIRED'))",
		"CHECK (expected_sha256 IS NULL OR expected_sha256 ~ '^[0-9A-Fa-f]{64}$')",
		"CHECK (actual_sha256 IS NULL OR actual_sha256 ~ '^[0-9A-Fa-f]{64}$')",
		"CHECK (actual_size IS NULL OR (actual_size >= 0 AND actual_size <= 21474836480))",
		"CHECK (version > 0)",
		"UNIQUE (session_id, part_number)",
		"CHECK (offset_bytes >= 0 AND size_bytes >= 0)",
		"FOREIGN KEY (session_id) REFERENCES filesystem.transfer_sessions(id)",
	} {
		require.Contains(t, up, invariant)
	}
	require.Contains(t, up, "ALTER TABLE filesystem.namespaces")
	require.Contains(t, up, "UNIQUE (workspace_id, id)")
	require.Contains(t, up, "ALTER TABLE filesystem.quota_reservations")
	require.Contains(t, up, "ADD COLUMN session_id UUID")
	require.Contains(t, up, "ADD COLUMN workspace_id UUID")
	require.Contains(t, up, "UPDATE filesystem.quota_reservations")
	require.Contains(t, up, "ALTER COLUMN workspace_id SET NOT NULL")
	require.Contains(t, up, "FOREIGN KEY (workspace_id, session_id)")
	require.Contains(t, up, "FOREIGN KEY (quota_account_id, workspace_id)")
	require.Contains(t, up, "UNIQUE (workspace_id, id)")
	require.Contains(t, up, "CREATE UNIQUE INDEX ux_quota_reservations_session")
	require.NotContains(t, up, "scope_type")
	require.NotContains(t, up, "scope_id")
	require.NotContains(t, up, "IF NOT EXISTS")
}

func TestFilesystemTransferMigrationDownReversesDependencies(t *testing.T) {
	up := readMigration(t, "000078_filesystem_transfer.up.sql")
	down := readMigration(t, "000078_filesystem_transfer.down.sql")
	require.Greater(t, strings.Index(up, "CREATE TABLE filesystem.transfer_sessions"), -1)
	requireInOrder(t, down,
		"DROP INDEX IF EXISTS filesystem.ux_quota_reservations_session",
		"DROP CONSTRAINT IF EXISTS fk_quota_reservations_session",
		"DROP COLUMN session_id",
		"DROP TABLE IF EXISTS filesystem.transfer_parts",
		"DROP TABLE IF EXISTS filesystem.transfer_sessions",
	)
	require.Contains(t, down, "DROP CONSTRAINT IF EXISTS fk_namespaces_workspace_id")
	require.Contains(t, down, "DROP CONSTRAINT IF EXISTS fk_transfer_sessions_namespace_workspace")
}

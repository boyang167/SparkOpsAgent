package migrations

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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
		"request_hash CHAR(64) NOT NULL",
		"checksum CHAR(64) CHECK (checksum IS NULL OR checksum ~ '^[0-9A-Fa-f]{64}$')",
		"UNIQUE (session_id, part_number)",
		"CHECK (offset_bytes >= 0 AND size_bytes > 0)",
		"etag TEXT NOT NULL CHECK (length(btrim(etag)) > 0)",
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
	require.Contains(t, up, "CREATE INDEX ix_transfer_sessions_workspace_status")
	require.Contains(t, up, "CREATE INDEX ix_transfer_sessions_expires_at")
	require.Contains(t, up, "CREATE INDEX ix_transfer_parts_session_offset")
	require.Contains(t, up, "CREATE OR REPLACE FUNCTION filesystem.validate_transfer_part_bounds()")
	require.Contains(t, up, "CREATE TRIGGER trg_transfer_parts_validate_bounds")
	require.Contains(t, up, "BEFORE INSERT OR UPDATE")
	require.Contains(t, up, "NEW.size_bytes > session_row.expected_size - NEW.offset_bytes")
	require.Contains(t, up, "RAISE EXCEPTION")
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
		"DROP TRIGGER IF EXISTS trg_transfer_parts_validate_bounds",
		"DROP FUNCTION IF EXISTS filesystem.validate_transfer_part_bounds()",
		"DROP TABLE IF EXISTS filesystem.transfer_parts",
		"DROP TABLE IF EXISTS filesystem.transfer_sessions",
	)
	require.Contains(t, down, "DROP CONSTRAINT IF EXISTS fk_namespaces_workspace_id")
	require.Contains(t, down, "DROP CONSTRAINT IF EXISTS fk_transfer_sessions_namespace_workspace")
}

func TestTransferPartBoundsAreEnforcedByPostgresWhenConfigured(t *testing.T) {
	db := openMigrationPostgres(t)
	require.NoError(t, db.Exec(readMigration(t, "000077_filesystem_kernel.up.sql")).Error)
	require.NoError(t, db.Exec(readMigration(t, "000078_filesystem_transfer.up.sql")).Error)

	workspaceID, namespaceID, organizationID, creatorID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO filesystem.namespaces (id, organization_id, workspace_id, status)
		 VALUES (?, ?, ?, 'ACTIVE')`,
		namespaceID, organizationID, workspaceID,
	).Error)
	sessionID := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO filesystem.transfer_sessions
		 (id, workspace_id, namespace_id, target_name, protocol, expected_size,
		  temporary_object_key, status, classification, idempotency_key, request_hash, expires_at, created_by)
		 VALUES (?, ?, ?, 'file.bin', 'DIRECT', 10, 'tmp/file', 'INITIATED',
		  'PUBLIC', 'idempotency-1', ?, ?, ?)`,
		sessionID, workspaceID, namespaceID, strings.Repeat("a", 64), time.Now().Add(time.Hour), creatorID,
	).Error)
	err := db.Exec(
		`INSERT INTO filesystem.transfer_parts
		 (id, session_id, part_number, offset_bytes, size_bytes, checksum, etag)
		 VALUES (?, ?, 1, 0, 1, '', 'etag-empty-checksum')`,
		uuid.New(), sessionID,
	).Error
	require.Error(t, err)
	err = db.Exec(
		`INSERT INTO filesystem.transfer_parts
		 (id, session_id, part_number, offset_bytes, size_bytes, etag)
		 VALUES (?, ?, 1, 9, 2, 'etag')`,
		uuid.New(), sessionID,
	).Error
	require.Error(t, err)
}

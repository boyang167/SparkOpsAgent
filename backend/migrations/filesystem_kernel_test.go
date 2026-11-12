package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemKernelMigrationDefinesCoreTablesAndInvariants(t *testing.T) {
	up := readMigration(t, "000077_filesystem_kernel.up.sql")

	for _, table := range []string{
		"namespaces", "nodes", "file_revisions", "blobs", "storage_replicas",
		"quota_accounts", "quota_reservations", "resource_grants", "file_links",
	} {
		require.Contains(t, up, "CREATE TABLE IF NOT EXISTS filesystem."+table)
	}
	require.Contains(t, up, "UNIQUE(namespace_id, parent_id, normalized_name)")
	require.Contains(t, up, "UNIQUE(dedup_domain, sha256, size)")
	require.Contains(t, up, "CHECK (node_type IN ('FILE','DIRECTORY'))")
	require.Contains(t, up, "fk_nodes_current_revision")
	require.Contains(t, up, "REFERENCES filesystem.file_revisions(id)")
}

func TestFilesystemKernelMigrationAddsCircularRevisionForeignKeyAfterTables(t *testing.T) {
	up := readMigration(t, "000077_filesystem_kernel.up.sql")

	require.Greater(t,
		strings.Index(up, "CREATE TABLE IF NOT EXISTS filesystem.file_revisions"),
		-1,
	)
	require.Greater(t,
		strings.Index(up, "ALTER TABLE filesystem.nodes"),
		strings.Index(up, "CREATE TABLE IF NOT EXISTS filesystem.file_revisions"),
	)
	require.Contains(t, readMigration(t, "000077_filesystem_kernel.down.sql"),
		"DROP TABLE IF EXISTS filesystem.nodes",
	)
}

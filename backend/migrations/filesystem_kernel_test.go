package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemKernelMigrationDefinesCoreTablesAndInvariants(t *testing.T) {
	up := readMigration(t, "000077_filesystem_kernel.up.sql")
	down := readMigration(t, "000077_filesystem_kernel.down.sql")

	for _, table := range []string{
		"namespaces", "nodes", "file_revisions", "blobs", "storage_replicas",
		"quota_accounts", "quota_reservations", "resource_grants", "file_links",
	} {
		require.Contains(t, up, "CREATE TABLE IF NOT EXISTS filesystem."+table)
	}
	require.Contains(t, up, "UNIQUE NULLS NOT DISTINCT (namespace_id, parent_id, normalized_name)")
	require.Contains(t, up, "UNIQUE(dedup_domain, sha256, size)")
	require.Contains(t, up, "CHECK (node_type IN ('FILE','DIRECTORY'))")
	require.Contains(t, up, "fk_nodes_current_revision")
	require.Contains(t, up, "fk_nodes_parent_namespace")
	require.Contains(t, up, "fk_namespaces_root_node")
	require.Contains(t, up, "UNIQUE(id, namespace_id)")
	require.Contains(t, up, "CHECK (revision_no > 0)")
	require.Contains(t, up, "CHECK (size >= 0)")
	require.Contains(t, up, "REFERENCES filesystem.file_revisions(id)")
	require.Contains(t, down, "DROP CONSTRAINT IF EXISTS fk_namespaces_root_node")
	require.Contains(t, down, "DROP CONSTRAINT IF EXISTS fk_nodes_parent_namespace")
}

func TestFilesystemKernelMigrationAddsCircularRevisionForeignKeyAfterTables(t *testing.T) {
	up := readMigration(t, "000077_filesystem_kernel.up.sql")
	down := readMigration(t, "000077_filesystem_kernel.down.sql")

	require.Greater(t,
		strings.Index(up, "CREATE TABLE IF NOT EXISTS filesystem.file_revisions"),
		-1,
	)
	require.Greater(t,
		strings.Index(up, "fk_nodes_current_revision"),
		strings.Index(up, "CREATE TABLE IF NOT EXISTS filesystem.file_revisions"),
	)
	require.Contains(t, up, "FOREIGN KEY (current_revision_id, id)")
	require.Contains(t, up, "REFERENCES filesystem.file_revisions(id, node_id)")
	requireInOrder(t, down,
		"DROP CONSTRAINT IF EXISTS fk_nodes_current_revision",
		"DROP TABLE IF EXISTS filesystem.file_links",
		"DROP TABLE IF EXISTS filesystem.nodes",
		"DROP TABLE IF EXISTS filesystem.namespaces",
	)
}

func TestFilesystemKernelMigrationDownIsReverseOfUpDependencies(t *testing.T) {
	up := readMigration(t, "000077_filesystem_kernel.up.sql")
	down := readMigration(t, "000077_filesystem_kernel.down.sql")

	require.Contains(t, up, "CREATE SCHEMA IF NOT EXISTS filesystem")
	require.Contains(t, down, "DROP SCHEMA IF EXISTS filesystem")
	for _, table := range []string{
		"file_links", "resource_grants", "quota_reservations", "quota_accounts",
		"storage_replicas", "file_revisions", "blobs", "nodes", "namespaces",
	} {
		require.Contains(t, down, "DROP TABLE IF EXISTS filesystem."+table)
	}
}

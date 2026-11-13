package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemStorageFabricMigrationDefinesSchema(t *testing.T) {
	up := readMigration(t, "000079_filesystem_storage_fabric.up.sql")
	for _, table := range []string{
		"storage_providers",
		"storage_pools",
		"storage_pool_members",
		"storage_bindings",
		"placement_policies",
		"mounts",
		"lifecycle_policies",
		"lifecycle_jobs",
	} {
		require.Contains(t, up, "CREATE TABLE filesystem."+table)
	}
	for _, invariant := range []string{
		"secret_ref TEXT NOT NULL CHECK (length(btrim(secret_ref)) > 0)",
		"CHECK (provider_type IN ('LOCAL','NFS','S3'))",
		"CHECK (mode IN ('PRIMARY','REPLICA','ARCHIVE','DRAINING'))",
		"CHECK (scope_type IN ('ORGANIZATION','WORKSPACE','PROJECT','USER'))",
		"CHECK (access_mode IN ('READ_ONLY','READ_WRITE'))",
		"CHECK (job_type IN ('MIGRATE','REPLICATE','ARCHIVE','GC'))",
		"CHECK (status IN ('PENDING','RUNNING','COMPLETED','FAILED','CANCELLED'))",
		"idempotency_key TEXT NOT NULL CHECK (length(btrim(idempotency_key)) > 0)",
		"UNIQUE (pool_id, provider_id)",
		"UNIQUE (organization_id, name, version)",
		"UNIQUE (namespace_id, mount_path)",
		"FOREIGN KEY (pool_id) REFERENCES filesystem.storage_pools(id)",
		"FOREIGN KEY (provider_id) REFERENCES filesystem.storage_providers(id)",
		"FOREIGN KEY (namespace_id) REFERENCES filesystem.namespaces(id)",
	} {
		require.Contains(t, up, invariant)
	}
	lower := strings.ToLower(up)
	require.NotContains(t, lower, "password")
	require.NotContains(t, lower, "access_key")
	require.NotContains(t, lower, "api_secret")
	require.NotContains(t, lower, "credential")
}

func TestFilesystemStorageFabricMigrationDownReversesDependencies(t *testing.T) {
	down := readMigration(t, "000079_filesystem_storage_fabric.down.sql")
	requireInOrder(t, down,
		"DROP TABLE IF EXISTS filesystem.lifecycle_jobs",
		"DROP TABLE IF EXISTS filesystem.lifecycle_policies",
		"DROP TABLE IF EXISTS filesystem.mounts",
		"DROP TABLE IF EXISTS filesystem.placement_policies",
		"DROP TABLE IF EXISTS filesystem.storage_bindings",
		"DROP TABLE IF EXISTS filesystem.storage_pool_members",
		"DROP TABLE IF EXISTS filesystem.storage_pools",
		"DROP TABLE IF EXISTS filesystem.storage_providers",
	)
}

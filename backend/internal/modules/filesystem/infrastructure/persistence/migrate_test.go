package persistence

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPostgresFallbackConstraintsMatchKernelInvariants(t *testing.T) {
	sql := strings.Join(postgresFallbackConstraints, "\n")
	for _, invariant := range []string{
		"(dedup_domain, sha256, size)", "(namespace_id, normalized_name)", "(namespace_id, parent_id, normalized_name)",
		"(node_id, revision_no)", "(blob_id, provider_id, object_key)", "fk_nodes_parent_namespace",
		"fk_nodes_current_revision", "fk_namespaces_root_node", "effect IN ('ALLOW','DENY')",
	} {
		require.Contains(t, sql, invariant)
	}
}

func TestPostgresFallbackConstraintCatalog(t *testing.T) {
	db := openFilesystemPostgres(t)
	require.NoError(t, AutoMigrate(db))
	for _, index := range []string{
		"ux_filesystem_nodes_root_name", "ux_filesystem_nodes_child_name", "ux_filesystem_blobs_dedup",
		"ux_filesystem_revisions_node_no", "ux_filesystem_replicas_identity", "ux_filesystem_grants_identity",
	} {
		var count int64
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM pg_indexes WHERE schemaname = 'filesystem' AND indexname = ?", index).Scan(&count).Error)
		require.Equal(t, int64(1), count, index)
	}
	for _, constraint := range []string{"fk_nodes_parent_namespace", "fk_nodes_current_revision", "fk_namespaces_root_node", "ck_filesystem_grant_values"} {
		var count int64
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM pg_constraint WHERE conname = ?", constraint).Scan(&count).Error)
		require.Equal(t, int64(1), count, constraint)
	}
}

type postgresNamedSQLite struct {
	gorm.Dialector
}

func (postgresNamedSQLite) Name() string { return "postgres" }

func TestAutoMigrateSkipsAfterSQLMigrationsApplied(t *testing.T) {
	db, err := gorm.Open(postgresNamedSQLite{
		Dialector: sqlite.Open("file:" + t.Name() + "?mode=memory&cache=shared"),
	}, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS information_schema").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS public").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, db.Exec("CREATE TABLE information_schema.tables (table_schema TEXT, table_name TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO information_schema.tables (table_schema, table_name) VALUES ('public', 'schema_migrations')").Error)
	require.NoError(t, db.Exec("CREATE TABLE public.schema_migrations (version INTEGER, dirty BOOLEAN)").Error)
	require.NoError(t, db.Exec("INSERT INTO public.schema_migrations (version, dirty) VALUES (77, false)").Error)

	require.NoError(t, AutoMigrate(db))

	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM filesystem.sqlite_master WHERE type = 'table'").Scan(&count).Error)
	require.Zero(t, count)
}

func TestSQLiteKernelConstraintsRejectInvalidWrites(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, AutoMigrate(db))
	for _, table := range []string{"quota_reservations", "resource_grants", "file_links"} {
		var count int64
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM filesystem.sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count).Error)
		require.Equal(t, int64(1), count, table)
	}

	namespaceID := uuid.New()
	require.NoError(t, db.Create(&namespaceRow{ID: namespaceID, OrganizationID: uuid.New(), WorkspaceID: uuid.New(), Status: "ACTIVE"}).Error)
	require.Error(t, db.Exec(
		"INSERT INTO filesystem.nodes (id, namespace_id, node_type, name, normalized_name, state, owner_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		uuid.New(), namespaceID, "INVALID", "bad", "bad", "ACTIVE", uuid.New(),
	).Error)
	require.Error(t, db.Exec(
		"INSERT INTO filesystem.nodes (id, namespace_id, node_type, name, normalized_name, state, owner_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		uuid.New(), uuid.New(), "FILE", "orphan", "orphan", "ACTIVE", uuid.New(),
	).Error)
	require.Error(t, db.Exec(
		"INSERT INTO filesystem.blobs (id, dedup_domain, sha256, size, ref_count, state) VALUES (?, ?, ?, ?, ?, ?)",
		uuid.New(), uuid.New(), "hash", -1, 0, "AVAILABLE",
	).Error)
	require.Error(t, db.Exec(
		"INSERT INTO filesystem.blobs (id, dedup_domain, sha256, size, ref_count, state) VALUES (?, ?, ?, ?, ?, ?)",
		uuid.New(), uuid.New(), "hash-2", 1, 0, "INVALID",
	).Error)
	require.Error(t, db.Exec(
		"INSERT INTO filesystem.nodes (id, namespace_id, node_type, name, normalized_name, state, owner_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		uuid.New(), namespaceID, "FILE", "bad-state", "bad-state", "INVALID", uuid.New(),
	).Error)

	nodeID := uuid.New()
	require.NoError(t, db.Exec(
		"INSERT INTO filesystem.nodes (id, namespace_id, node_type, name, normalized_name, state, owner_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		nodeID, namespaceID, "FILE", "valid", "valid", "ACTIVE", uuid.New(),
	).Error)
	require.Error(t, db.Exec("UPDATE filesystem.namespaces SET root_node_id = ? WHERE id = ?", uuid.New(), namespaceID).Error)
	require.Error(t, db.Exec("UPDATE filesystem.nodes SET current_revision_id = ? WHERE id = ?", uuid.New(), nodeID).Error)
	require.Error(t, db.Exec(
		"INSERT INTO filesystem.file_revisions (id, node_id, revision_no, blob_id, size, mime_type, created_by) VALUES (?, ?, ?, ?, ?, ?, ?)",
		uuid.New(), nodeID, 0, uuid.New(), 0, "text/plain", uuid.New(),
	).Error)
	require.Error(t, db.Exec(
		"INSERT INTO filesystem.storage_replicas (id, blob_id, provider_id, object_key, state) VALUES (?, ?, ?, ?, ?)",
		uuid.New(), uuid.New(), uuid.New(), "bad", "INVALID",
	).Error)
	require.Error(t, db.Exec(
		"INSERT INTO filesystem.resource_grants (id, node_id, grantee_id, permission, effect) VALUES (?, ?, ?, ?, ?)",
		uuid.New(), nodeID, uuid.New(), "READ", "INVALID",
	).Error)

	var violations []struct {
		Table string
		RowID int
	}
	require.NoError(t, db.Raw("PRAGMA filesystem.foreign_key_check").Scan(&violations).Error)
	require.Empty(t, violations)
}

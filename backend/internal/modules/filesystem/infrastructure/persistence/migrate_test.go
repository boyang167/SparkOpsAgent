package persistence

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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

	var violations []struct {
		Table string
		RowID int
	}
	require.NoError(t, db.Raw("PRAGMA filesystem.foreign_key_check").Scan(&violations).Error)
	require.Empty(t, violations)
}

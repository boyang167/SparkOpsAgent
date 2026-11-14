package persistence

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

func TestSQLiteAutoMigrateUpgradesLegacyGrantRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	orgID, namespaceID, directoryID, granteeID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	restrictedSessionID, confidentialSessionID, newerSessionID := uuid.New(), uuid.New(), uuid.New()
	restrictedNodeID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-transfer-node:"+restrictedSessionID.String()))
	confidentialNodeID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-transfer-node:"+confidentialSessionID.String()))
	unknownFileID := uuid.New()
	require.NoError(t, db.Exec(`CREATE TABLE filesystem.namespaces (
		id BLOB PRIMARY KEY, namespace_type TEXT, organization_id BLOB, workspace_id BLOB,
		owner_user_id BLOB, root_node_id BLOB, status TEXT, policy_version INTEGER,
		created_at DATETIME, updated_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE filesystem.nodes (
		id BLOB PRIMARY KEY, namespace_id BLOB, parent_id BLOB, node_type TEXT,
		name TEXT, normalized_name TEXT, current_revision_id BLOB, state TEXT,
		owner_id BLOB, created_at DATETIME, updated_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE filesystem.transfer_sessions (
		id BLOB PRIMARY KEY, workspace_id BLOB, status TEXT NOT NULL, classification TEXT NOT NULL,
		committed_node_id BLOB, created_at DATETIME, expires_at DATETIME,
		idempotency_key TEXT, expected_size INTEGER
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE filesystem.export_tasks (
		id BLOB PRIMARY KEY
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE filesystem.resource_grants (
		id BLOB PRIMARY KEY, node_id BLOB NOT NULL, grantee_id BLOB NOT NULL,
		permission TEXT NOT NULL, effect TEXT NOT NULL, expires_at DATETIME,
		created_at DATETIME, UNIQUE(node_id, grantee_id, permission, effect)
	)`).Error)
	require.NoError(t, db.Exec("INSERT INTO filesystem.namespaces (id, namespace_type, organization_id, workspace_id, status) VALUES (?, 'WORKSPACE', ?, ?, 'ACTIVE')", namespaceID, orgID, uuid.New()).Error)
	require.NoError(t, db.Exec("INSERT INTO filesystem.nodes (id, namespace_id, node_type, name, normalized_name, state, owner_id) VALUES (?, ?, 'DIRECTORY', 'root', 'root', 'ACTIVE', ?)", directoryID, namespaceID, uuid.New()).Error)
	for _, node := range []struct {
		id   uuid.UUID
		name string
	}{{restrictedNodeID, "restricted.bin"}, {confidentialNodeID, "confidential.bin"}, {unknownFileID, "unknown.bin"}} {
		require.NoError(t, db.Exec("INSERT INTO filesystem.nodes (id, namespace_id, node_type, name, normalized_name, state, owner_id) VALUES (?, ?, 'FILE', ?, ?, 'ACTIVE', ?)", node.id, namespaceID, node.name, node.name, uuid.New()).Error)
	}
	require.NoError(t, db.Exec("INSERT INTO filesystem.transfer_sessions (id, status, classification) VALUES (?, 'COMPLETED', 'RESTRICTED')", restrictedSessionID).Error)
	require.NoError(t, db.Exec("INSERT INTO filesystem.transfer_sessions (id, status, classification) VALUES (?, 'COMPLETED', 'PUBLIC')", confidentialSessionID).Error)
	require.NoError(t, db.Exec("INSERT INTO filesystem.transfer_sessions (id, status, classification, committed_node_id) VALUES (?, 'COMPLETED', 'CONFIDENTIAL', ?)", newerSessionID, confidentialNodeID).Error)
	require.NoError(t, db.Exec("INSERT INTO filesystem.resource_grants (id, node_id, grantee_id, permission, effect) VALUES (?, ?, ?, 'READ', 'ALLOW')", uuid.New(), directoryID, granteeID).Error)

	require.NoError(t, AutoMigrate(db))
	var got struct {
		OrganizationID uuid.UUID
		PrincipalType  model.PrincipalType
		PrincipalID    uuid.UUID
		Inherit        bool `gorm:"column:inherit_to_children"`
		PolicyVersion  int64
	}
	require.NoError(t, db.Table("filesystem.resource_grants").Select("organization_id, principal_type, principal_id, inherit_to_children, policy_version").Where("node_id = ?", directoryID).Scan(&got).Error)
	require.Equal(t, orgID, got.OrganizationID)
	require.Equal(t, model.PrincipalUser, got.PrincipalType)
	require.Equal(t, granteeID, got.PrincipalID)
	require.True(t, got.Inherit)
	require.EqualValues(t, 1, got.PolicyVersion)
	for nodeID, want := range map[uuid.UUID]model.TransferClassification{
		directoryID: model.TransferClassificationInternal, restrictedNodeID: model.TransferClassificationRestricted,
		confidentialNodeID: model.TransferClassificationConfidential, unknownFileID: model.TransferClassificationRestricted,
	} {
		var classification model.TransferClassification
		require.NoError(t, db.Table("filesystem.nodes").Select("classification").Where("id = ?", nodeID).Scan(&classification).Error)
		require.Equal(t, want, classification, nodeID)
	}
	exportID := uuid.New()
	require.NoError(t, db.Exec("INSERT INTO filesystem.export_tasks (id) VALUES (?)", exportID).Error)
	var exportClassification model.TransferClassification
	require.NoError(t, db.Table("filesystem.export_tasks").Select("classification").Where("id = ?", exportID).Scan(&exportClassification).Error)
	require.Equal(t, model.TransferClassificationRestricted, exportClassification)
	var classificationColumn struct {
		NotNull int `gorm:"column:notnull"`
	}
	require.NoError(t, db.Raw(`SELECT "notnull" FROM filesystem.pragma_table_info('nodes') WHERE name = 'classification'`).Scan(&classificationColumn).Error)
	require.Equal(t, 1, classificationColumn.NotNull)

	repo := NewRepository(db)
	grants, err := repo.ListResourceGrants(context.Background(), []uuid.UUID{directoryID}, []repository.PrincipalRef{{Type: model.PrincipalUser, ID: granteeID}}, model.ResourcePermissionRead, time.Now())
	require.NoError(t, err)
	require.Len(t, grants, 1)
	grants, err = repo.ListResourceGrants(context.Background(), []uuid.UUID{directoryID}, []repository.PrincipalRef{{Type: model.PrincipalWorkspace, ID: granteeID}}, model.ResourcePermissionRead, time.Now())
	require.NoError(t, err)
	require.Empty(t, grants)
}

func TestSQLiteAutoMigrateRebuildsPartiallyUpgradedGrantRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	orgID, namespaceID, nodeID, granteeID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, db.Exec(`CREATE TABLE filesystem.namespaces (
		id BLOB PRIMARY KEY, namespace_type TEXT, organization_id BLOB, workspace_id BLOB,
		owner_user_id BLOB, root_node_id BLOB, status TEXT, policy_version INTEGER,
		created_at DATETIME, updated_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE filesystem.nodes (
		id BLOB PRIMARY KEY, namespace_id BLOB, parent_id BLOB, node_type TEXT,
		name TEXT, normalized_name TEXT, current_revision_id BLOB, state TEXT,
		owner_id BLOB, created_at DATETIME, updated_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE filesystem.resource_grants (
		id BLOB PRIMARY KEY, node_id BLOB NOT NULL, organization_id BLOB,
		grantee_id BLOB NOT NULL, principal_type TEXT, principal_id BLOB,
		permission TEXT NOT NULL, effect TEXT NOT NULL, inherit_to_children INTEGER,
		expires_at DATETIME, revoked_at DATETIME, policy_version INTEGER, created_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec("INSERT INTO filesystem.namespaces (id, namespace_type, organization_id, workspace_id, status) VALUES (?, 'WORKSPACE', ?, ?, 'ACTIVE')", namespaceID, orgID, uuid.New()).Error)
	require.NoError(t, db.Exec("INSERT INTO filesystem.nodes (id, namespace_id, node_type, name, normalized_name, state, owner_id) VALUES (?, ?, 'DIRECTORY', 'root', 'root', 'ACTIVE', ?)", nodeID, namespaceID, uuid.New()).Error)
	require.NoError(t, db.Exec("INSERT INTO filesystem.resource_grants (id, node_id, organization_id, grantee_id, principal_type, principal_id, permission, effect) VALUES (?, ?, ?, ?, 'USER', ?, 'READ', 'ALLOW')", uuid.New(), nodeID, orgID, granteeID, granteeID).Error)

	require.NoError(t, AutoMigrate(db))
	var columns []struct {
		Name    string `gorm:"column:name"`
		NotNull int    `gorm:"column:notnull"`
	}
	require.NoError(t, db.Raw(`SELECT name, "notnull" FROM filesystem.pragma_table_info('resource_grants') WHERE name IN ('organization_id', 'principal_type', 'principal_id')`).Scan(&columns).Error)
	require.Len(t, columns, 3)
	for _, column := range columns {
		require.Equal(t, 1, column.NotNull, column.Name)
	}
	var got struct {
		OrganizationID uuid.UUID
		PrincipalType  model.PrincipalType
		PrincipalID    uuid.UUID
	}
	require.NoError(t, db.Table("filesystem.resource_grants").Select("organization_id, principal_type, principal_id").Where("node_id = ?", nodeID).Scan(&got).Error)
	require.Equal(t, orgID, got.OrganizationID)
	require.Equal(t, model.PrincipalUser, got.PrincipalType)
	require.Equal(t, granteeID, got.PrincipalID)

	repo := NewRepository(db)
	grants, err := repo.ListResourceGrants(context.Background(), []uuid.UUID{nodeID}, []repository.PrincipalRef{{Type: model.PrincipalUser, ID: granteeID}}, model.ResourcePermissionRead, time.Now())
	require.NoError(t, err)
	require.Len(t, grants, 1)
	grants, err = repo.ListResourceGrants(context.Background(), []uuid.UUID{nodeID}, []repository.PrincipalRef{{Type: model.PrincipalRole, ID: granteeID}}, model.ResourcePermissionRead, time.Now())
	require.NoError(t, err)
	require.Empty(t, grants)
}

func TestPostgresFallbackConstraintsMatchKernelInvariants(t *testing.T) {
	fallbackSQL := strings.Join(postgresFallbackConstraints, "\n")
	migrationBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "migrations", "000077_filesystem_kernel.up.sql"))
	require.NoError(t, err)
	migrationSQL := string(migrationBytes)
	for _, invariant := range []struct {
		migration string
		fallback  string
	}{
		{"UNIQUE(dedup_domain, sha256, size)", "(dedup_domain, sha256, size)"},
		{"UNIQUE(node_id, revision_no)", "(node_id, revision_no)"},
		{"UNIQUE(blob_id, provider_id, object_key)", "(blob_id, provider_id, object_key)"},
		{"fk_nodes_parent_namespace", "fk_nodes_parent_namespace"},
		{"fk_nodes_current_revision", "fk_nodes_current_revision"},
		{"fk_namespaces_root_node", "fk_namespaces_root_node"},
		{"namespace_id UUID NOT NULL REFERENCES filesystem.namespaces(id)", "fk_nodes_namespace"},
		{"node_id UUID NOT NULL REFERENCES filesystem.nodes(id)", "fk_revisions_node"},
		{"blob_id UUID NOT NULL REFERENCES filesystem.blobs(id)", "fk_revisions_blob"},
		{"blob_id UUID NOT NULL REFERENCES filesystem.blobs(id)", "fk_replicas_blob"},
		{"quota_account_id UUID NOT NULL REFERENCES filesystem.quota_accounts(id)", "fk_reservations_quota"},
		{"node_id UUID NOT NULL REFERENCES filesystem.nodes(id)", "fk_grants_node"},
		{"node_id UUID NOT NULL REFERENCES filesystem.nodes(id)", "fk_file_links_node"},
		{"revision_id UUID REFERENCES filesystem.file_revisions(id)", "fk_file_links_revision"},
		{"node_type VARCHAR(16) NOT NULL CHECK (node_type IN ('FILE','DIRECTORY'))", "ck_filesystem_node_type"},
		{"state VARCHAR(32) NOT NULL CHECK (state IN ('ACTIVE'", "ck_filesystem_node_state"},
		{"state VARCHAR(32) NOT NULL CHECK (state IN ('PENDING'", "ck_filesystem_blob_state"},
		{"state VARCHAR(32) NOT NULL CHECK (state IN ('CREATING'", "ck_filesystem_replica_state"},
		{"revision_no INT NOT NULL CHECK (revision_no > 0)", "ck_filesystem_revision_values"},
		{"bytes BIGINT NOT NULL CHECK (bytes >= 0)", "ck_filesystem_reservation_values"},
		{"effect VARCHAR(8) NOT NULL CHECK (effect IN ('ALLOW','DENY'))", "effect IN ('ALLOW','DENY')"},
		{"used_bytes BIGINT NOT NULL DEFAULT 0 CHECK (used_bytes >= 0)", "ck_filesystem_quota_values"},
	} {
		require.Contains(t, migrationSQL, invariant.migration)
		require.Contains(t, fallbackSQL, invariant.fallback)
	}
}

func TestPostgresFallbackLineageBindsNamespacesToOrganization(t *testing.T) {
	fallbackSQL := strings.Join(postgresFallbackConstraints, "\n")
	for _, fragment := range []string{
		"ux_filesystem_namespaces_id_organization",
		"FOREIGN KEY (source_namespace_id, organization_id) REFERENCES filesystem.namespaces(id, organization_id)",
		"FOREIGN KEY (target_namespace_id, organization_id) REFERENCES filesystem.namespaces(id, organization_id)",
	} {
		require.Contains(t, fallbackSQL, fragment)
	}
}

func TestPostgresClassificationFallbackIsFailClosedAndTransferAware(t *testing.T) {
	for _, fragment := range []string{
		"node_type = 'DIRECTORY'",
		"node_type = 'FILE'",
		"classification = 'RESTRICTED'",
		"status = 'COMPLETED'",
		"MAX(CASE classification",
		"committed_node_id",
	} {
		require.Contains(t, postgresNodeClassificationBackfill, fragment)
	}
}

func TestPostgresFallbackUsesNamespaceTransferForeignKey(t *testing.T) {
	fallbackSQL := strings.Join(postgresFallbackConstraints, "\n")
	transferForeignKey := "FOREIGN KEY (namespace_id) REFERENCES filesystem.namespaces(id)"
	require.Contains(t, fallbackSQL, transferForeignKey)

	migrationSQL, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "migrations", "000086_filesystem_namespace_quota.up.sql"))
	require.NoError(t, err)
	normalizedMigrationSQL := strings.Join(strings.Fields(string(migrationSQL)), " ")
	require.Contains(t, normalizedMigrationSQL, transferForeignKey)
	require.Contains(t, normalizedMigrationSQL, "DROP CONSTRAINT IF EXISTS fk_transfer_sessions_namespace_workspace")
}

func TestPostgresFallbackConstraintCatalog(t *testing.T) {
	db := openFilesystemPostgres(t)
	require.NoError(t, AutoMigrate(db))
	for _, index := range []string{
		"ux_filesystem_namespaces_workspace_id_id", "ux_filesystem_nodes_root_name", "ux_filesystem_nodes_child_name", "ux_filesystem_blobs_dedup",
		"ux_filesystem_revisions_node_no", "ux_filesystem_replicas_identity", "ux_filesystem_grants_active_identity",
		"ux_transfer_sessions_workspace_id", "ux_transfer_sessions_idempotency",
		"ux_transfer_parts_number", "ux_quota_reservations_session",
		"ix_transfer_sessions_workspace_status", "ix_transfer_sessions_expires_at", "ix_transfer_parts_session_offset",
	} {
		var count int64
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM pg_indexes WHERE schemaname = 'filesystem' AND indexname = ?", index).Scan(&count).Error)
		require.Equal(t, int64(1), count, index)
	}
	for _, constraint := range []string{
		"fk_nodes_namespace", "fk_nodes_parent_namespace", "fk_nodes_current_revision", "fk_namespaces_root_node",
		"fk_revisions_node", "fk_revisions_blob", "fk_replicas_blob", "fk_reservations_quota",
		"fk_grants_node", "fk_file_links_node", "fk_file_links_revision",
		"ck_filesystem_node_type", "ck_filesystem_node_state", "ck_filesystem_blob_size", "ck_filesystem_blob_state",
		"ck_filesystem_revision_values", "ck_filesystem_replica_state", "ck_filesystem_reservation_values",
		"ck_filesystem_grant_values", "ck_filesystem_quota_values",
		"fk_transfer_namespace_workspace", "fk_transfer_parent_namespace", "fk_transfer_parts_session",
		"fk_reservation_transfer", "fk_reservation_account_workspace",
		"ck_transfer_protocol", "ck_transfer_expected_size", "ck_transfer_actual_size",
		"ck_transfer_status", "ck_transfer_classification", "ck_transfer_version",
		"ck_transfer_expected_sha", "ck_transfer_actual_sha", "ck_transfer_request_hash",
		"ck_transfer_part_values", "ck_transfer_part_sha",
	} {
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
	for _, table := range []string{"transfer_sessions", "transfer_parts", "quota_reservations", "resource_grants", "file_links"} {
		var count int64
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM filesystem.sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count).Error)
		require.Equal(t, int64(1), count, table)
	}
	for _, index := range []string{
		"ix_transfer_sessions_workspace_status",
		"ix_transfer_sessions_expires_at",
		"ix_transfer_parts_session_offset",
	} {
		var count int64
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM filesystem.sqlite_master WHERE type = 'index' AND name = ?", index).Scan(&count).Error)
		require.Equal(t, int64(1), count, index)
	}

	namespaceID, workspaceID := uuid.New(), uuid.New()
	require.NoError(t, db.Create(&namespaceRow{ID: namespaceID, OrganizationID: uuid.New(), WorkspaceID: &workspaceID, Status: "ACTIVE"}).Error)
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
	require.Error(t, db.Exec(
		"INSERT INTO filesystem.transfer_sessions (id, workspace_id, namespace_id, target_name, protocol, expected_size, temporary_object_key, status, classification, idempotency_key, request_hash, expires_at, version, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		uuid.New(), uuid.New(), namespaceID, "bad", "INVALID", 1, "temp", "INITIATED", "PUBLIC", "bad", strings.Repeat("a", 64), time.Now().Add(time.Hour), 1, uuid.New(), time.Now(), time.Now(),
	).Error)
	sessionID := uuid.New()
	require.NoError(t, db.Exec(
		"INSERT INTO filesystem.transfer_sessions (id, workspace_id, namespace_id, target_name, protocol, expected_size, temporary_object_key, status, classification, idempotency_key, request_hash, expires_at, version, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		sessionID, workspaceID, namespaceID, "valid", "DIRECT", 10, "temp", "INITIATED", "PUBLIC", "valid", strings.Repeat("a", 64), time.Now().Add(time.Hour), 1, uuid.New(), time.Now(), time.Now(),
	).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO filesystem.transfer_parts (id, session_id, part_number, offset_bytes, size_bytes, etag, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		uuid.New(), sessionID, 1, 0, 1, "etag-null-checksum", time.Now(), time.Now(),
	).Error)
	require.Error(t, db.Exec(
		"INSERT INTO filesystem.transfer_parts (id, session_id, part_number, offset_bytes, size_bytes, checksum, etag, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		uuid.New(), sessionID, 2, 1, 1, "", "etag-empty-checksum", time.Now(), time.Now(),
	).Error)
	require.Error(t, db.Exec(
		"INSERT INTO filesystem.transfer_parts (id, session_id, part_number, offset_bytes, size_bytes, etag, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		uuid.New(), sessionID, 3, 9, 2, "etag", time.Now(), time.Now(),
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
		"INSERT INTO filesystem.resource_grants (id, node_id, organization_id, grantee_id, principal_type, principal_id, permission, effect) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		uuid.New(), nodeID, uuid.New(), uuid.New(), "USER", uuid.New(), "READ", "INVALID",
	).Error)

	var violations []struct {
		Table string
		RowID int
	}
	require.NoError(t, db.Raw("PRAGMA filesystem.foreign_key_check").Scan(&violations).Error)
	require.Empty(t, violations)
}

func TestSQLiteAutoMigrateCreatesLineageWithCompositeForeignKeys(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, AutoMigrate(db))

	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM filesystem.sqlite_master WHERE type = 'table' AND name = 'file_lineage'").Scan(&count).Error)
	require.Equal(t, int64(1), count)

	orgID, otherOrgID, sourceNamespaceID, targetNamespaceID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	sourceNodeID, targetNodeID, otherNodeID := uuid.New(), uuid.New(), uuid.New()
	sourceRevisionID, targetRevisionID, otherRevisionID := uuid.New(), uuid.New(), uuid.New()
	blobID, ownerID := uuid.New(), uuid.New()
	require.NoError(t, db.Exec(
		"INSERT INTO filesystem.namespaces (id, namespace_type, organization_id, owner_user_id, status, policy_version) VALUES (?, 'USER', ?, ?, 'ACTIVE', 1)",
		sourceNamespaceID, orgID, ownerID,
	).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO filesystem.namespaces (id, namespace_type, organization_id, status, policy_version) VALUES (?, 'ORGANIZATION', ?, 'ACTIVE', 1)",
		targetNamespaceID, orgID,
	).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO filesystem.blobs (id, dedup_domain, sha256, size, ref_count, state) VALUES (?, ?, ?, 1, 3, 'AVAILABLE')",
		blobID, orgID, strings.Repeat("a", 64),
	).Error)
	for _, node := range []struct {
		id          uuid.UUID
		namespaceID uuid.UUID
		name        string
	}{
		{sourceNodeID, sourceNamespaceID, "source"},
		{targetNodeID, targetNamespaceID, "target"},
		{otherNodeID, sourceNamespaceID, "other"},
	} {
		require.NoError(t, db.Exec(
			"INSERT INTO filesystem.nodes (id, namespace_id, node_type, name, normalized_name, state, owner_id, classification) VALUES (?, ?, 'FILE', ?, ?, 'ACTIVE', ?, 'INTERNAL')",
			node.id, node.namespaceID, node.name, node.name, ownerID,
		).Error)
	}
	for _, revision := range []struct {
		id     uuid.UUID
		nodeID uuid.UUID
	}{
		{sourceRevisionID, sourceNodeID},
		{targetRevisionID, targetNodeID},
		{otherRevisionID, otherNodeID},
	} {
		require.NoError(t, db.Exec(
			"INSERT INTO filesystem.file_revisions (id, node_id, revision_no, blob_id, size, mime_type, created_by) VALUES (?, ?, 1, ?, 1, 'text/plain', ?)",
			revision.id, revision.nodeID, blobID, ownerID,
		).Error)
	}
	insert := `INSERT INTO filesystem.file_lineage
		(id, organization_id, source_namespace_id, source_node_id, source_revision_id,
		 target_namespace_id, target_node_id, target_revision_id, relation, command_id, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'ADOPTED_AS_ORGANIZATION_FILE', ?, ?, ?)`
	require.Error(t, db.Exec(insert,
		uuid.New(), orgID, sourceNamespaceID, sourceNodeID, otherRevisionID,
		targetNamespaceID, targetNodeID, targetRevisionID, uuid.New(), ownerID, time.Now(),
	).Error)
	require.Error(t, db.Exec(insert,
		uuid.New(), orgID, targetNamespaceID, sourceNodeID, sourceRevisionID,
		targetNamespaceID, targetNodeID, targetRevisionID, uuid.New(), ownerID, time.Now(),
	).Error)
	require.Error(t, db.Exec(insert,
		uuid.New(), otherOrgID, sourceNamespaceID, sourceNodeID, sourceRevisionID,
		targetNamespaceID, targetNodeID, targetRevisionID, uuid.New(), ownerID, time.Now(),
	).Error)
	require.NoError(t, db.Exec(insert,
		uuid.New(), orgID, sourceNamespaceID, sourceNodeID, sourceRevisionID,
		targetNamespaceID, targetNodeID, targetRevisionID, uuid.New(), ownerID, time.Now(),
	).Error)
}

func TestSQLiteAutoMigrateRepairsMissingLineageTableOnPartialUpgrade(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS filesystem").Error)
	require.NoError(t, AutoMigrate(db))
	require.NoError(t, db.Exec("DROP TABLE filesystem.file_lineage").Error)
	require.NoError(t, db.Exec(`CREATE TABLE filesystem.file_lineage (
		id BLOB PRIMARY KEY,
		organization_id BLOB NOT NULL,
		source_namespace_id BLOB NOT NULL,
		source_node_id BLOB NOT NULL,
		source_revision_id BLOB NOT NULL,
		target_namespace_id BLOB NOT NULL,
		target_node_id BLOB NOT NULL UNIQUE,
		target_revision_id BLOB NOT NULL,
		relation TEXT NOT NULL,
		command_id BLOB NOT NULL UNIQUE,
		created_by BLOB NOT NULL,
		created_at DATETIME NOT NULL
	)`).Error)

	require.NoError(t, AutoMigrate(db))
	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM filesystem.sqlite_master WHERE type = 'table' AND name = 'file_lineage'").Scan(&count).Error)
	require.Equal(t, int64(1), count)
	var tableSQL string
	require.NoError(t, db.Raw("SELECT sql FROM filesystem.sqlite_master WHERE type = 'table' AND name = 'file_lineage'").Scan(&tableSQL).Error)
	require.Contains(t, tableSQL, "FOREIGN KEY(source_namespace_id, organization_id)")
	require.Contains(t, tableSQL, "FOREIGN KEY(target_namespace_id, organization_id)")
}

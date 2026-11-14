package migrations

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestFilesystemFileLineageMigrationDefinesCompositeOwnershipConstraintsAndSafeDown(t *testing.T) {
	up := strings.Join(strings.Fields(readMigration(t, "000087_filesystem_file_lineage.up.sql")), " ")
	down := readMigration(t, "000087_filesystem_file_lineage.down.sql")

	for _, fragment := range []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_namespaces_id_organization ON filesystem.namespaces(id, organization_id)",
		"FOREIGN KEY (source_namespace_id, organization_id) REFERENCES filesystem.namespaces(id, organization_id)",
		"FOREIGN KEY (target_namespace_id, organization_id) REFERENCES filesystem.namespaces(id, organization_id)",
		"FOREIGN KEY (source_node_id, source_namespace_id) REFERENCES filesystem.nodes(id, namespace_id)",
		"FOREIGN KEY (target_node_id, target_namespace_id) REFERENCES filesystem.nodes(id, namespace_id)",
		"FOREIGN KEY (source_revision_id, source_node_id) REFERENCES filesystem.file_revisions(id, node_id)",
		"FOREIGN KEY (target_revision_id, target_node_id) REFERENCES filesystem.file_revisions(id, node_id)",
		"UNIQUE",
		"command_id",
	} {
		require.Contains(t, up, fragment)
	}
	require.Contains(t, down, "cannot roll back active filesystem file lineage")
	require.Contains(t, down, "IF EXISTS (SELECT 1 FROM filesystem.file_lineage)")
	require.Less(t, strings.Index(down, "DROP TABLE filesystem.file_lineage"), strings.Index(down, "DROP INDEX filesystem.ux_filesystem_namespaces_id_organization"))
}

func TestFilesystemFileLineageMigrationUpDownAndForeignKeysWhenPostgresConfigured(t *testing.T) {
	db := openMigrationPostgres(t)
	require.NoError(t, db.Exec(readMigration(t, "000077_filesystem_kernel.up.sql")).Error)
	require.NoError(t, db.Exec(readMigration(t, "000083_filesystem_namespace_types.up.sql")).Error)
	require.NoError(t, db.Exec(readMigration(t, "000087_filesystem_file_lineage.up.sql")).Error)

	orgID, otherOrgID, sourceNamespaceID, targetNamespaceID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	sourceNodeID, targetNodeID, wrongNodeID := uuid.New(), uuid.New(), uuid.New()
	sourceRevisionID, targetRevisionID, wrongRevisionID := uuid.New(), uuid.New(), uuid.New()
	blobID, ownerID := uuid.New(), uuid.New()
	require.NoError(t, db.Exec(`
		INSERT INTO filesystem.namespaces (id, namespace_type, organization_id, status)
		VALUES (?, 'ORGANIZATION', ?, 'ACTIVE'), (?, 'ORGANIZATION', ?, 'ACTIVE')
	`, sourceNamespaceID, orgID, targetNamespaceID, orgID).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO filesystem.blobs (id, dedup_domain, sha256, size, ref_count, state, created_at)
		VALUES (?, ?, ?, 1, 3, 'AVAILABLE', NOW())
	`, blobID, orgID, strings.Repeat("a", 64)).Error)
	for _, node := range []struct {
		id          uuid.UUID
		namespaceID uuid.UUID
		name        string
	}{
		{sourceNodeID, sourceNamespaceID, "source"},
		{targetNodeID, targetNamespaceID, "target"},
		{wrongNodeID, sourceNamespaceID, "wrong"},
	} {
		require.NoError(t, db.Exec(`
			INSERT INTO filesystem.nodes
				(id, namespace_id, node_type, name, normalized_name, state, owner_id, classification, created_at, updated_at)
			VALUES (?, ?, 'FILE', ?, ?, 'ACTIVE', ?, 'INTERNAL', NOW(), NOW())
		`, node.id, node.namespaceID, node.name, node.name, ownerID).Error)
	}
	for _, revision := range []struct {
		id     uuid.UUID
		nodeID uuid.UUID
	}{
		{sourceRevisionID, sourceNodeID},
		{targetRevisionID, targetNodeID},
		{wrongRevisionID, wrongNodeID},
	} {
		require.NoError(t, db.Exec(`
			INSERT INTO filesystem.file_revisions
				(id, node_id, revision_no, blob_id, size, mime_type, created_by, created_at)
			VALUES (?, ?, 1, ?, 1, 'text/plain', ?, NOW())
		`, revision.id, revision.nodeID, blobID, ownerID).Error)
	}
	require.Error(t, db.Exec(`
		INSERT INTO filesystem.file_lineage
			(id, organization_id, source_namespace_id, source_node_id, source_revision_id,
			 target_namespace_id, target_node_id, target_revision_id, relation, command_id, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'ADOPTED_AS_ORGANIZATION_FILE', ?, ?, ?)
	`, uuid.New(), orgID, sourceNamespaceID, sourceNodeID, wrongRevisionID,
		targetNamespaceID, targetNodeID, targetRevisionID, uuid.New(), ownerID, time.Now()).Error)

	lineageID := uuid.New()
	require.Error(t, db.Exec(`
		INSERT INTO filesystem.file_lineage
			(id, organization_id, source_namespace_id, source_node_id, source_revision_id,
			 target_namespace_id, target_node_id, target_revision_id, relation, command_id, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'ADOPTED_AS_ORGANIZATION_FILE', ?, ?, ?)
	`, uuid.New(), otherOrgID, sourceNamespaceID, sourceNodeID, sourceRevisionID,
		targetNamespaceID, targetNodeID, targetRevisionID, uuid.New(), ownerID, time.Now()).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO filesystem.file_lineage
			(id, organization_id, source_namespace_id, source_node_id, source_revision_id,
			 target_namespace_id, target_node_id, target_revision_id, relation, command_id, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'ADOPTED_AS_ORGANIZATION_FILE', ?, ?, ?)
	`, lineageID, orgID, sourceNamespaceID, sourceNodeID, sourceRevisionID,
		targetNamespaceID, targetNodeID, targetRevisionID, uuid.New(), ownerID, time.Now()).Error)

	require.ErrorContains(t, db.Exec(readMigration(t, "000087_filesystem_file_lineage.down.sql")).Error, "cannot roll back active filesystem file lineage")
	var count int64
	require.NoError(t, db.Table("filesystem.file_lineage").Where("id = ?", lineageID).Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, db.Exec("DELETE FROM filesystem.file_lineage WHERE id = ?", lineageID).Error)
	require.NoError(t, db.Exec(readMigration(t, "000087_filesystem_file_lineage.down.sql")).Error)
	require.Error(t, db.Exec("SELECT 1 FROM filesystem.file_lineage").Error)
}

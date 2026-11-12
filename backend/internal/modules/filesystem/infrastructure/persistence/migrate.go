package persistence

import (
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if !postgres.ShouldRunAutoMigrate(db) {
		return nil
	}
	if db.Dialector.Name() == "sqlite" {
		for _, statement := range []string{
			`CREATE TABLE IF NOT EXISTS filesystem.namespaces (id BLOB PRIMARY KEY, organization_id BLOB NOT NULL, workspace_id BLOB NOT NULL UNIQUE, root_node_id BLOB, status TEXT NOT NULL, created_at DATETIME, updated_at DATETIME)`,
			`CREATE TABLE IF NOT EXISTS filesystem.nodes (id BLOB PRIMARY KEY, namespace_id BLOB NOT NULL, parent_id BLOB, node_type TEXT NOT NULL, name TEXT NOT NULL, normalized_name TEXT NOT NULL, current_revision_id BLOB, state TEXT NOT NULL, owner_id BLOB NOT NULL, created_at DATETIME, updated_at DATETIME, UNIQUE(id, namespace_id), FOREIGN KEY(namespace_id) REFERENCES namespaces(id), FOREIGN KEY(parent_id, namespace_id) REFERENCES nodes(id, namespace_id))`,
			`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_nodes_root_name ON nodes(namespace_id, normalized_name) WHERE parent_id IS NULL`,
			`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_nodes_child_name ON nodes(namespace_id, parent_id, normalized_name) WHERE parent_id IS NOT NULL`,
			`CREATE TABLE IF NOT EXISTS filesystem.blobs (id BLOB PRIMARY KEY, dedup_domain BLOB NOT NULL, sha256 TEXT NOT NULL, size INTEGER NOT NULL, encryption_key_ref TEXT, ref_count INTEGER NOT NULL, state TEXT NOT NULL, created_at DATETIME)`,
			`CREATE TABLE IF NOT EXISTS filesystem.file_revisions (id BLOB PRIMARY KEY, node_id BLOB NOT NULL, revision_no INTEGER NOT NULL, blob_id BLOB NOT NULL, size INTEGER NOT NULL, mime_type TEXT NOT NULL, created_by BLOB NOT NULL, created_at DATETIME, UNIQUE(node_id, revision_no), FOREIGN KEY(node_id) REFERENCES nodes(id), FOREIGN KEY(blob_id) REFERENCES blobs(id))`,
			`CREATE TABLE IF NOT EXISTS filesystem.storage_replicas (id BLOB PRIMARY KEY, blob_id BLOB NOT NULL, provider_id BLOB NOT NULL, object_key TEXT NOT NULL, checksum TEXT, state TEXT NOT NULL, created_at DATETIME, UNIQUE(blob_id, provider_id, object_key), FOREIGN KEY(blob_id) REFERENCES blobs(id))`,
		} {
			if err := db.Exec(statement).Error; err != nil {
				return wrapPersistence(err)
			}
		}
		return nil
	}
	if postgres.ShouldRunAutoMigrate(db) && db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS filesystem").Error; err != nil {
			return wrapPersistence(err)
		}
	}
	return wrapPersistence(db.AutoMigrate(&namespaceRow{}, &nodeRow{}, &blobRow{}, &revisionRow{}, &replicaRow{}))
}

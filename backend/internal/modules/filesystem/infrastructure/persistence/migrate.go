package persistence

import (
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"gorm.io/gorm"
)

var postgresFallbackConstraints = []string{
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_nodes_id_namespace ON filesystem.nodes(id, namespace_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_nodes_root_name ON filesystem.nodes(namespace_id, normalized_name) WHERE parent_id IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_nodes_child_name ON filesystem.nodes(namespace_id, parent_id, normalized_name) WHERE parent_id IS NOT NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_blobs_dedup ON filesystem.blobs(dedup_domain, sha256, size)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_revisions_node_no ON filesystem.file_revisions(node_id, revision_no)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_revisions_id_node ON filesystem.file_revisions(id, node_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_replicas_identity ON filesystem.storage_replicas(blob_id, provider_id, object_key)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_grants_identity ON filesystem.resource_grants(node_id, grantee_id, permission, effect)`,
	`DO $$ BEGIN ALTER TABLE filesystem.nodes ADD CONSTRAINT fk_nodes_namespace FOREIGN KEY (namespace_id) REFERENCES filesystem.namespaces(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.nodes ADD CONSTRAINT fk_nodes_parent_namespace FOREIGN KEY (parent_id, namespace_id) REFERENCES filesystem.nodes(id, namespace_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.nodes ADD CONSTRAINT fk_nodes_current_revision FOREIGN KEY (current_revision_id, id) REFERENCES filesystem.file_revisions(id, node_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.namespaces ADD CONSTRAINT fk_namespaces_root_node FOREIGN KEY (root_node_id, id) REFERENCES filesystem.nodes(id, namespace_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_revisions ADD CONSTRAINT fk_revisions_node FOREIGN KEY (node_id) REFERENCES filesystem.nodes(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_revisions ADD CONSTRAINT fk_revisions_blob FOREIGN KEY (blob_id) REFERENCES filesystem.blobs(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.storage_replicas ADD CONSTRAINT fk_replicas_blob FOREIGN KEY (blob_id) REFERENCES filesystem.blobs(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.quota_reservations ADD CONSTRAINT fk_reservations_quota FOREIGN KEY (quota_account_id) REFERENCES filesystem.quota_accounts(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.resource_grants ADD CONSTRAINT fk_grants_node FOREIGN KEY (node_id) REFERENCES filesystem.nodes(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_links ADD CONSTRAINT fk_file_links_node FOREIGN KEY (node_id) REFERENCES filesystem.nodes(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_links ADD CONSTRAINT fk_file_links_revision FOREIGN KEY (revision_id) REFERENCES filesystem.file_revisions(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.nodes ADD CONSTRAINT ck_filesystem_node_type CHECK (node_type IN ('FILE','DIRECTORY')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.nodes ADD CONSTRAINT ck_filesystem_node_state CHECK (state IN ('ACTIVE','TRASHED','RETENTION_HOLD','PURGE_PENDING','PURGED')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.blobs ADD CONSTRAINT ck_filesystem_blob_size CHECK (size >= 0 AND ref_count >= 0); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.blobs ADD CONSTRAINT ck_filesystem_blob_state CHECK (state IN ('PENDING','AVAILABLE','QUARANTINED','TOMBSTONED','GC_PENDING','DELETED')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_revisions ADD CONSTRAINT ck_filesystem_revision_values CHECK (revision_no > 0 AND size >= 0); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.storage_replicas ADD CONSTRAINT ck_filesystem_replica_state CHECK (state IN ('CREATING','VERIFYING','AVAILABLE','DRAINING','FAILED','DELETED')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.quota_reservations ADD CONSTRAINT ck_filesystem_reservation_values CHECK (bytes >= 0 AND files >= 0); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.resource_grants ADD CONSTRAINT ck_filesystem_grant_values CHECK (permission IN ('READ','WRITE','DELETE','SHARE','ADMIN') AND effect IN ('ALLOW','DENY')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.quota_accounts ADD CONSTRAINT ck_filesystem_quota_values CHECK (used_bytes >= 0 AND reserved_bytes >= 0 AND used_files >= 0 AND reserved_files >= 0 AND (max_bytes IS NULL OR max_bytes >= 0) AND (max_files IS NULL OR max_files >= 0)); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
}

func AutoMigrate(db *gorm.DB) error {
	if !postgres.ShouldRunAutoMigrate(db) {
		return nil
	}
	if db.Dialector.Name() == "sqlite" {
		for _, statement := range []string{
			`CREATE TABLE IF NOT EXISTS filesystem.namespaces (id BLOB PRIMARY KEY, organization_id BLOB NOT NULL, workspace_id BLOB NOT NULL UNIQUE, root_node_id BLOB, status TEXT NOT NULL, created_at DATETIME, updated_at DATETIME, FOREIGN KEY(root_node_id, id) REFERENCES nodes(id, namespace_id))`,
			`CREATE TABLE IF NOT EXISTS filesystem.nodes (id BLOB PRIMARY KEY, namespace_id BLOB NOT NULL, parent_id BLOB, node_type TEXT NOT NULL CHECK (node_type IN ('FILE','DIRECTORY')), name TEXT NOT NULL, normalized_name TEXT NOT NULL, current_revision_id BLOB, state TEXT NOT NULL CHECK (state IN ('ACTIVE','TRASHED','RETENTION_HOLD','PURGE_PENDING','PURGED')), owner_id BLOB NOT NULL, created_at DATETIME, updated_at DATETIME, UNIQUE(id, namespace_id), FOREIGN KEY(namespace_id) REFERENCES namespaces(id), FOREIGN KEY(parent_id, namespace_id) REFERENCES nodes(id, namespace_id), FOREIGN KEY(current_revision_id, id) REFERENCES file_revisions(id, node_id))`,
			`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_nodes_root_name ON nodes(namespace_id, normalized_name) WHERE parent_id IS NULL`,
			`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_nodes_child_name ON nodes(namespace_id, parent_id, normalized_name) WHERE parent_id IS NOT NULL`,
			`CREATE TABLE IF NOT EXISTS filesystem.blobs (id BLOB PRIMARY KEY, dedup_domain BLOB NOT NULL, sha256 TEXT NOT NULL, size INTEGER NOT NULL CHECK (size >= 0), encryption_key_ref TEXT, ref_count INTEGER NOT NULL CHECK (ref_count >= 0), state TEXT NOT NULL CHECK (state IN ('PENDING','AVAILABLE','QUARANTINED','TOMBSTONED','GC_PENDING','DELETED')), created_at DATETIME)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_blobs_dedup ON blobs(dedup_domain, sha256, size)`,
			`CREATE TABLE IF NOT EXISTS filesystem.file_revisions (id BLOB PRIMARY KEY, node_id BLOB NOT NULL, revision_no INTEGER NOT NULL CHECK (revision_no > 0), blob_id BLOB NOT NULL, size INTEGER NOT NULL CHECK (size >= 0), mime_type TEXT NOT NULL, created_by BLOB NOT NULL, created_at DATETIME, UNIQUE(id, node_id), UNIQUE(node_id, revision_no), FOREIGN KEY(node_id) REFERENCES nodes(id), FOREIGN KEY(blob_id) REFERENCES blobs(id))`,
			`CREATE TABLE IF NOT EXISTS filesystem.storage_replicas (id BLOB PRIMARY KEY, blob_id BLOB NOT NULL, provider_id BLOB NOT NULL, object_key TEXT NOT NULL, checksum TEXT, state TEXT NOT NULL CHECK (state IN ('CREATING','VERIFYING','AVAILABLE','DRAINING','FAILED','DELETED')), verified_at DATETIME, created_at DATETIME, UNIQUE(blob_id, provider_id, object_key), FOREIGN KEY(blob_id) REFERENCES blobs(id))`,
			`CREATE TABLE IF NOT EXISTS filesystem.quota_accounts (id BLOB PRIMARY KEY, organization_id BLOB NOT NULL, workspace_id BLOB NOT NULL UNIQUE, used_bytes INTEGER NOT NULL DEFAULT 0 CHECK (used_bytes >= 0), reserved_bytes INTEGER NOT NULL DEFAULT 0 CHECK (reserved_bytes >= 0), used_files INTEGER NOT NULL DEFAULT 0 CHECK (used_files >= 0), reserved_files INTEGER NOT NULL DEFAULT 0 CHECK (reserved_files >= 0), max_bytes INTEGER CHECK (max_bytes IS NULL OR max_bytes >= 0), max_files INTEGER CHECK (max_files IS NULL OR max_files >= 0), version INTEGER NOT NULL DEFAULT 0, updated_at DATETIME)`,
			`CREATE TABLE IF NOT EXISTS filesystem.quota_reservations (id BLOB PRIMARY KEY, quota_account_id BLOB NOT NULL, bytes INTEGER NOT NULL CHECK (bytes >= 0), files INTEGER NOT NULL CHECK (files >= 0), state TEXT NOT NULL, expires_at DATETIME NOT NULL, created_at DATETIME, FOREIGN KEY(quota_account_id) REFERENCES quota_accounts(id))`,
			`CREATE TABLE IF NOT EXISTS filesystem.resource_grants (id BLOB PRIMARY KEY, node_id BLOB NOT NULL, grantee_id BLOB NOT NULL, permission TEXT NOT NULL CHECK (permission IN ('READ','WRITE','DELETE','SHARE','ADMIN')), effect TEXT NOT NULL CHECK (effect IN ('ALLOW','DENY')), expires_at DATETIME, created_at DATETIME, UNIQUE(node_id, grantee_id, permission, effect), FOREIGN KEY(node_id) REFERENCES nodes(id))`,
			`CREATE TABLE IF NOT EXISTS filesystem.file_links (id BLOB PRIMARY KEY, node_id BLOB NOT NULL, revision_id BLOB, link_type TEXT NOT NULL, link_id BLOB NOT NULL, created_at DATETIME, UNIQUE(link_type, link_id, node_id, revision_id), FOREIGN KEY(node_id) REFERENCES nodes(id), FOREIGN KEY(revision_id) REFERENCES file_revisions(id))`,
			`CREATE TABLE IF NOT EXISTS filesystem.command_dedup (command_id BLOB PRIMARY KEY, workspace_id BLOB NOT NULL, operation TEXT NOT NULL, request_hash TEXT NOT NULL, node_id BLOB, revision_id BLOB, state TEXT NOT NULL, created_at DATETIME, updated_at DATETIME)`,
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
	if err := db.AutoMigrate(&namespaceRow{}, &nodeRow{}, &blobRow{}, &revisionRow{}, &replicaRow{}, &quotaAccountRow{}, &quotaReservationRow{}, &resourceGrantRow{}, &fileLinkRow{}, &commandDedupRow{}); err != nil {
		return wrapPersistence(err)
	}
	if db.Dialector.Name() == "postgres" {
		for _, statement := range postgresFallbackConstraints {
			if err := db.Exec(statement).Error; err != nil {
				return wrapPersistence(err)
			}
		}
	}
	return nil
}

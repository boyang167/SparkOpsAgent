package persistence

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"gorm.io/gorm"
)

const postgresNodeClassificationBackfill = `
UPDATE filesystem.nodes SET classification = 'INTERNAL' WHERE node_type = 'DIRECTORY';
UPDATE filesystem.nodes SET classification = 'RESTRICTED' WHERE node_type = 'FILE';
CREATE EXTENSION IF NOT EXISTS pgcrypto;
WITH transfer_hashes AS (
	SELECT id, encode(digest(
		uuid_send('6ba7b811-9dad-11d1-80b4-00c04fd430c8'::uuid)
		|| convert_to('filesystem-transfer-node:' || id::text, 'UTF8'), 'sha1'
	), 'hex') AS hash
	FROM filesystem.transfer_sessions WHERE status = 'COMPLETED'
), transfer_nodes AS (
	SELECT id, (
		substr(hash, 1, 8) || '-' || substr(hash, 9, 4) || '-5' || substr(hash, 14, 3) || '-'
		|| CASE lower(substr(hash, 17, 1))
			WHEN '0' THEN '8' WHEN '1' THEN '8' WHEN '2' THEN '8' WHEN '3' THEN '8'
			WHEN '4' THEN '9' WHEN '5' THEN '9' WHEN '6' THEN '9' WHEN '7' THEN '9'
			WHEN '8' THEN 'a' WHEN '9' THEN 'a' WHEN 'a' THEN 'a' WHEN 'b' THEN 'a'
			ELSE 'b' END
		|| substr(hash, 18, 3) || '-' || substr(hash, 21, 12)
	)::uuid AS node_id FROM transfer_hashes
)
UPDATE filesystem.transfer_sessions AS session
SET committed_node_id = transfer_nodes.node_id
FROM transfer_nodes JOIN filesystem.nodes ON filesystem.nodes.id = transfer_nodes.node_id
WHERE session.id = transfer_nodes.id AND session.committed_node_id IS NULL;
WITH classified_nodes AS (
	SELECT committed_node_id,
		MAX(CASE classification
			WHEN 'PUBLIC' THEN 1 WHEN 'INTERNAL' THEN 2
			WHEN 'CONFIDENTIAL' THEN 3 WHEN 'RESTRICTED' THEN 4 END) AS sensitivity
	FROM filesystem.transfer_sessions
	WHERE status = 'COMPLETED' AND committed_node_id IS NOT NULL
	GROUP BY committed_node_id
)
UPDATE filesystem.nodes AS node SET classification = CASE classified_nodes.sensitivity
	WHEN 1 THEN 'PUBLIC' WHEN 2 THEN 'INTERNAL'
	WHEN 3 THEN 'CONFIDENTIAL' WHEN 4 THEN 'RESTRICTED' END
FROM classified_nodes WHERE node.id = classified_nodes.committed_node_id;
`

var postgresFallbackConstraints = []string{
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_nodes_id_namespace ON filesystem.nodes(id, namespace_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_namespaces_workspace_id_id ON filesystem.namespaces(workspace_id, id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_namespaces_workspace_id ON filesystem.namespaces(workspace_id) WHERE namespace_type = 'WORKSPACE'`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_namespaces_organization_id ON filesystem.namespaces(organization_id) WHERE namespace_type = 'ORGANIZATION'`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_namespaces_user_owner ON filesystem.namespaces(organization_id, owner_user_id) WHERE namespace_type = 'USER'`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_quota_accounts_namespace ON filesystem.quota_accounts(namespace_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_quota_accounts_id_namespace ON filesystem.quota_accounts(id, namespace_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_nodes_root_name ON filesystem.nodes(namespace_id, normalized_name) WHERE parent_id IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_nodes_child_name ON filesystem.nodes(namespace_id, parent_id, normalized_name) WHERE parent_id IS NOT NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_blobs_dedup ON filesystem.blobs(dedup_domain, sha256, size)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_revisions_node_no ON filesystem.file_revisions(node_id, revision_no)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_revisions_id_node ON filesystem.file_revisions(id, node_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_replicas_identity ON filesystem.storage_replicas(blob_id, provider_id, object_key)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_grants_active_identity ON filesystem.resource_grants(node_id, principal_type, principal_id, permission, effect) WHERE revoked_at IS NULL`,
	`DO $$ BEGIN ALTER TABLE filesystem.nodes ADD CONSTRAINT fk_nodes_namespace FOREIGN KEY (namespace_id) REFERENCES filesystem.namespaces(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.nodes ADD CONSTRAINT fk_nodes_parent_namespace FOREIGN KEY (parent_id, namespace_id) REFERENCES filesystem.nodes(id, namespace_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.nodes ADD CONSTRAINT fk_nodes_current_revision FOREIGN KEY (current_revision_id, id) REFERENCES filesystem.file_revisions(id, node_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.namespaces ADD CONSTRAINT fk_namespaces_root_node FOREIGN KEY (root_node_id, id) REFERENCES filesystem.nodes(id, namespace_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_revisions ADD CONSTRAINT fk_revisions_node FOREIGN KEY (node_id) REFERENCES filesystem.nodes(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_revisions ADD CONSTRAINT fk_revisions_blob FOREIGN KEY (blob_id) REFERENCES filesystem.blobs(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.storage_replicas ADD CONSTRAINT fk_replicas_blob FOREIGN KEY (blob_id) REFERENCES filesystem.blobs(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.quota_reservations ADD CONSTRAINT fk_reservations_quota FOREIGN KEY (quota_account_id) REFERENCES filesystem.quota_accounts(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.quota_accounts ADD CONSTRAINT fk_quota_accounts_namespace FOREIGN KEY (namespace_id) REFERENCES filesystem.namespaces(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.resource_grants ADD CONSTRAINT fk_grants_node FOREIGN KEY (node_id) REFERENCES filesystem.nodes(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_links ADD CONSTRAINT fk_file_links_node FOREIGN KEY (node_id) REFERENCES filesystem.nodes(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_links ADD CONSTRAINT fk_file_links_revision FOREIGN KEY (revision_id) REFERENCES filesystem.file_revisions(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_lineage ADD CONSTRAINT fk_file_lineage_source_namespace FOREIGN KEY (source_namespace_id) REFERENCES filesystem.namespaces(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_lineage ADD CONSTRAINT fk_file_lineage_source_node_namespace FOREIGN KEY (source_node_id, source_namespace_id) REFERENCES filesystem.nodes(id, namespace_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_lineage ADD CONSTRAINT fk_file_lineage_source_revision_node FOREIGN KEY (source_revision_id, source_node_id) REFERENCES filesystem.file_revisions(id, node_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_lineage ADD CONSTRAINT fk_file_lineage_target_namespace FOREIGN KEY (target_namespace_id) REFERENCES filesystem.namespaces(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_lineage ADD CONSTRAINT fk_file_lineage_target_node_namespace FOREIGN KEY (target_node_id, target_namespace_id) REFERENCES filesystem.nodes(id, namespace_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_lineage ADD CONSTRAINT fk_file_lineage_target_revision_node FOREIGN KEY (target_revision_id, target_node_id) REFERENCES filesystem.file_revisions(id, node_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_file_lineage_target_node ON filesystem.file_lineage(target_node_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_file_lineage_command ON filesystem.file_lineage(command_id)`,
	`CREATE INDEX IF NOT EXISTS ix_file_lineage_source ON filesystem.file_lineage(organization_id, source_node_id, created_at DESC)`,
	`DO $$ BEGIN ALTER TABLE filesystem.artifacts ADD CONSTRAINT fk_artifacts_file FOREIGN KEY (file_id) REFERENCES filesystem.nodes(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.nodes ADD CONSTRAINT ck_filesystem_node_type CHECK (node_type IN ('FILE','DIRECTORY')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.namespaces ADD CONSTRAINT ck_filesystem_namespace_type CHECK (namespace_type IN ('ORGANIZATION','WORKSPACE','USER')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.namespaces ADD CONSTRAINT ck_filesystem_namespace_scope CHECK ((namespace_type = 'ORGANIZATION' AND workspace_id IS NULL AND owner_user_id IS NULL) OR (namespace_type = 'WORKSPACE' AND workspace_id IS NOT NULL AND owner_user_id IS NULL) OR (namespace_type = 'USER' AND workspace_id IS NULL AND owner_user_id IS NOT NULL)); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.nodes ADD CONSTRAINT ck_filesystem_node_state CHECK (state IN ('ACTIVE','TRASHED','RETENTION_HOLD','PURGE_PENDING','PURGED')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.nodes ADD CONSTRAINT ck_filesystem_node_classification CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.blobs ADD CONSTRAINT ck_filesystem_blob_size CHECK (size >= 0 AND ref_count >= 0); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.blobs ADD CONSTRAINT ck_filesystem_blob_state CHECK (state IN ('PENDING','AVAILABLE','QUARANTINED','TOMBSTONED','GC_PENDING','DELETED')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.file_revisions ADD CONSTRAINT ck_filesystem_revision_values CHECK (revision_no > 0 AND size >= 0); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.storage_replicas ADD CONSTRAINT ck_filesystem_replica_state CHECK (state IN ('CREATING','VERIFYING','AVAILABLE','DRAINING','FAILED','DELETED')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.quota_reservations ADD CONSTRAINT ck_filesystem_reservation_values CHECK (bytes >= 0 AND files >= 0); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.resource_grants ADD CONSTRAINT ck_filesystem_grant_values CHECK (permission IN ('READ','DOWNLOAD','WRITE','DELETE','SHARE','ADMIN') AND effect IN ('ALLOW','DENY')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`CREATE INDEX IF NOT EXISTS ix_filesystem_share_requests_org_status ON filesystem.share_requests(organization_id, status, created_at)`,
	`CREATE INDEX IF NOT EXISTS ix_filesystem_share_requests_node ON filesystem.share_requests(node_id, created_at)`,
	`DO $$ BEGIN ALTER TABLE filesystem.share_requests ADD CONSTRAINT ck_filesystem_share_request_status CHECK (status IN ('PENDING','APPROVED','REJECTED','CANCELLED')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_sessions ADD CONSTRAINT fk_transfer_committed_node FOREIGN KEY (committed_node_id) REFERENCES filesystem.nodes(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.export_tasks ADD CONSTRAINT ck_filesystem_export_classification CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.quota_accounts ADD CONSTRAINT ck_filesystem_quota_values CHECK (used_bytes >= 0 AND reserved_bytes >= 0 AND used_files >= 0 AND reserved_files >= 0 AND (max_bytes IS NULL OR max_bytes >= 0) AND (max_files IS NULL OR max_files >= 0)); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_transfer_sessions_workspace_id ON filesystem.transfer_sessions(workspace_id, id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_transfer_sessions_idempotency ON filesystem.transfer_sessions(workspace_id, idempotency_key)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_transfer_sessions_namespace_idempotency ON filesystem.transfer_sessions(namespace_id, idempotency_key)`,
	`CREATE INDEX IF NOT EXISTS ix_transfer_sessions_workspace_status ON filesystem.transfer_sessions(workspace_id, status)`,
	`CREATE INDEX IF NOT EXISTS ix_transfer_sessions_workspace_created ON filesystem.transfer_sessions(workspace_id, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS ix_transfer_sessions_workspace_status_created ON filesystem.transfer_sessions(workspace_id, status, created_at DESC, id DESC)`,
	`CREATE INDEX IF NOT EXISTS ix_transfer_sessions_expires_at ON filesystem.transfer_sessions(expires_at)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_transfer_parts_number ON filesystem.transfer_parts(session_id, part_number)`,
	`CREATE INDEX IF NOT EXISTS ix_transfer_parts_session_offset ON filesystem.transfer_parts(session_id, offset_bytes)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_quota_reservations_session ON filesystem.quota_reservations(session_id) WHERE session_id IS NOT NULL`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_sessions ADD CONSTRAINT fk_transfer_namespace FOREIGN KEY (namespace_id) REFERENCES filesystem.namespaces(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_sessions ADD CONSTRAINT fk_transfer_parent_namespace FOREIGN KEY (target_parent_id, namespace_id) REFERENCES filesystem.nodes(id, namespace_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_parts ADD CONSTRAINT fk_transfer_parts_session FOREIGN KEY (session_id) REFERENCES filesystem.transfer_sessions(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.quota_reservations ADD CONSTRAINT fk_reservation_transfer FOREIGN KEY (session_id) REFERENCES filesystem.transfer_sessions(id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.quota_reservations ADD CONSTRAINT fk_reservation_account_namespace FOREIGN KEY (quota_account_id, namespace_id) REFERENCES filesystem.quota_accounts(id, namespace_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_sessions ADD CONSTRAINT ck_transfer_protocol CHECK (protocol IN ('DIRECT','MULTIPART')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_sessions ADD CONSTRAINT ck_transfer_expected_size CHECK (expected_size >= 0 AND expected_size <= 21474836480); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_sessions ADD CONSTRAINT ck_transfer_actual_size CHECK (actual_size IS NULL OR (actual_size >= 0 AND actual_size <= 21474836480)); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_sessions ADD CONSTRAINT ck_transfer_status CHECK (status IN ('INITIATED','UPLOADING','PAUSED','VERIFYING','SCANNING','COMMITTING','COMPLETED','FAILED','REJECTED','ABORTED','EXPIRED')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_sessions ADD CONSTRAINT ck_transfer_classification CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_sessions ADD CONSTRAINT ck_transfer_version CHECK (version > 0); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_sessions ADD CONSTRAINT ck_transfer_expected_sha CHECK (expected_sha256 IS NULL OR expected_sha256 ~ '^[0-9A-Fa-f]{64}$'); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_sessions ADD CONSTRAINT ck_transfer_actual_sha CHECK (actual_sha256 IS NULL OR actual_sha256 ~ '^[0-9A-Fa-f]{64}$'); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_sessions ADD CONSTRAINT ck_transfer_request_hash CHECK (request_hash ~ '^[0-9A-Fa-f]{64}$'); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_parts ADD CONSTRAINT ck_transfer_part_values CHECK (part_number > 0 AND offset_bytes >= 0 AND size_bytes > 0); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.transfer_parts ADD CONSTRAINT ck_transfer_part_sha CHECK (checksum IS NULL OR checksum ~ '^[0-9A-Fa-f]{64}$'); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`CREATE OR REPLACE FUNCTION filesystem.validate_transfer_part_bounds() RETURNS TRIGGER LANGUAGE plpgsql AS $$ DECLARE expected BIGINT; BEGIN SELECT expected_size INTO expected FROM filesystem.transfer_sessions WHERE id = NEW.session_id; IF NOT FOUND THEN RAISE EXCEPTION 'transfer session does not exist' USING ERRCODE = '23503'; END IF; IF NEW.offset_bytes > expected OR NEW.size_bytes > expected - NEW.offset_bytes THEN RAISE EXCEPTION 'transfer part exceeds expected session size' USING ERRCODE = '23514'; END IF; RETURN NEW; END; $$`,
	`DROP TRIGGER IF EXISTS trg_transfer_parts_validate_bounds ON filesystem.transfer_parts`,
	`CREATE TRIGGER trg_transfer_parts_validate_bounds BEFORE INSERT OR UPDATE ON filesystem.transfer_parts FOR EACH ROW EXECUTE FUNCTION filesystem.validate_transfer_part_bounds()`,
	`CREATE UNIQUE INDEX IF NOT EXISTS ux_storage_providers_org_name ON filesystem.storage_providers(organization_id, name)`,
	`DO $$ BEGIN ALTER TABLE filesystem.storage_providers ADD CONSTRAINT ck_storage_provider_type CHECK (provider_type IN ('LOCAL','NFS','S3')); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.storage_providers ADD CONSTRAINT ck_storage_provider_secret CHECK (length(btrim(secret_ref)) > 0); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	`DO $$ BEGIN ALTER TABLE filesystem.storage_providers ADD CONSTRAINT ck_storage_provider_version CHECK (config_version > 0); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
}

func AutoMigrate(db *gorm.DB) error {
	if !postgres.ShouldRunAutoMigrate(db) {
		return nil
	}
	if db.Dialector.Name() == "sqlite" {
		for _, statement := range []string{
			`CREATE TABLE IF NOT EXISTS filesystem.namespaces (id BLOB PRIMARY KEY, namespace_type TEXT NOT NULL CHECK (namespace_type IN ('ORGANIZATION','WORKSPACE','USER')), organization_id BLOB NOT NULL, workspace_id BLOB, owner_user_id BLOB, root_node_id BLOB, status TEXT NOT NULL, policy_version INTEGER NOT NULL DEFAULT 1, created_at DATETIME, updated_at DATETIME, UNIQUE(workspace_id, id), CHECK ((namespace_type = 'ORGANIZATION' AND workspace_id IS NULL AND owner_user_id IS NULL) OR (namespace_type = 'WORKSPACE' AND workspace_id IS NOT NULL AND owner_user_id IS NULL) OR (namespace_type = 'USER' AND workspace_id IS NULL AND owner_user_id IS NOT NULL)), FOREIGN KEY(root_node_id, id) REFERENCES nodes(id, namespace_id))`,
			`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_namespaces_workspace_id ON namespaces(workspace_id) WHERE namespace_type = 'WORKSPACE'`,
			`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_namespaces_organization_id ON namespaces(organization_id) WHERE namespace_type = 'ORGANIZATION'`,
			`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_namespaces_user_owner ON namespaces(organization_id, owner_user_id) WHERE namespace_type = 'USER'`,
			`CREATE TABLE IF NOT EXISTS filesystem.nodes (id BLOB PRIMARY KEY, namespace_id BLOB NOT NULL, parent_id BLOB, node_type TEXT NOT NULL CHECK (node_type IN ('FILE','DIRECTORY')), name TEXT NOT NULL, normalized_name TEXT NOT NULL, current_revision_id BLOB, state TEXT NOT NULL CHECK (state IN ('ACTIVE','TRASHED','RETENTION_HOLD','PURGE_PENDING','PURGED')), owner_id BLOB NOT NULL, classification TEXT NOT NULL DEFAULT 'INTERNAL' CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED')), created_at DATETIME, updated_at DATETIME, UNIQUE(id, namespace_id), FOREIGN KEY(namespace_id) REFERENCES namespaces(id), FOREIGN KEY(parent_id, namespace_id) REFERENCES nodes(id, namespace_id), FOREIGN KEY(current_revision_id, id) REFERENCES file_revisions(id, node_id))`,
			`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_nodes_root_name ON nodes(namespace_id, normalized_name) WHERE parent_id IS NULL`,
			`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_nodes_child_name ON nodes(namespace_id, parent_id, normalized_name) WHERE parent_id IS NOT NULL`,
			`CREATE TABLE IF NOT EXISTS filesystem.blobs (id BLOB PRIMARY KEY, dedup_domain BLOB NOT NULL, sha256 TEXT NOT NULL, size INTEGER NOT NULL CHECK (size >= 0), encryption_key_ref TEXT, ref_count INTEGER NOT NULL CHECK (ref_count >= 0), state TEXT NOT NULL CHECK (state IN ('PENDING','AVAILABLE','QUARANTINED','TOMBSTONED','GC_PENDING','DELETED')), preferred_provider_id BLOB, tombstoned_at DATETIME, legal_hold INTEGER NOT NULL DEFAULT 0 CHECK (legal_hold IN (0, 1)), created_at DATETIME)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_blobs_dedup ON blobs(dedup_domain, sha256, size)`,
			`CREATE TABLE IF NOT EXISTS filesystem.file_revisions (id BLOB PRIMARY KEY, node_id BLOB NOT NULL, revision_no INTEGER NOT NULL CHECK (revision_no > 0), blob_id BLOB NOT NULL, size INTEGER NOT NULL CHECK (size >= 0), mime_type TEXT NOT NULL, created_by BLOB NOT NULL, created_at DATETIME, UNIQUE(id, node_id), UNIQUE(node_id, revision_no), FOREIGN KEY(node_id) REFERENCES nodes(id), FOREIGN KEY(blob_id) REFERENCES blobs(id))`,
			`CREATE TABLE IF NOT EXISTS filesystem.storage_replicas (id BLOB PRIMARY KEY, blob_id BLOB NOT NULL, provider_id BLOB NOT NULL, object_key TEXT NOT NULL, checksum TEXT, state TEXT NOT NULL CHECK (state IN ('CREATING','VERIFYING','AVAILABLE','DRAINING','FAILED','DELETED')), verified_at DATETIME, delete_after DATETIME, created_at DATETIME, UNIQUE(blob_id, provider_id, object_key), FOREIGN KEY(blob_id) REFERENCES blobs(id))`,
			`CREATE TABLE IF NOT EXISTS filesystem.replica_migrations (blob_id BLOB NOT NULL, target_provider_id BLOB NOT NULL, target_replica_id BLOB NOT NULL, source_provider_id BLOB NOT NULL, bytes_copied INTEGER NOT NULL DEFAULT 0, target_object_key TEXT NOT NULL, policy_version INTEGER NOT NULL, updated_at DATETIME NOT NULL, PRIMARY KEY (blob_id, target_provider_id), FOREIGN KEY(blob_id) REFERENCES blobs(id))`,
			`CREATE TABLE IF NOT EXISTS filesystem.quota_accounts (id BLOB PRIMARY KEY, organization_id BLOB NOT NULL, namespace_id BLOB NOT NULL UNIQUE, workspace_id BLOB UNIQUE, used_bytes INTEGER NOT NULL DEFAULT 0 CHECK (used_bytes >= 0), reserved_bytes INTEGER NOT NULL DEFAULT 0 CHECK (reserved_bytes >= 0), used_files INTEGER NOT NULL DEFAULT 0 CHECK (used_files >= 0), reserved_files INTEGER NOT NULL DEFAULT 0 CHECK (reserved_files >= 0), max_bytes INTEGER CHECK (max_bytes IS NULL OR max_bytes >= 0), max_files INTEGER CHECK (max_files IS NULL OR max_files >= 0), version INTEGER NOT NULL DEFAULT 0, updated_at DATETIME, UNIQUE(id, namespace_id), FOREIGN KEY(namespace_id) REFERENCES namespaces(id))`,
			`CREATE TABLE IF NOT EXISTS filesystem.transfer_sessions (id BLOB PRIMARY KEY, workspace_id BLOB, namespace_id BLOB NOT NULL, target_parent_id BLOB, target_name TEXT NOT NULL CHECK (length(trim(target_name)) > 0 AND instr(target_name, '/') = 0 AND instr(target_name, char(92)) = 0 AND instr(target_name, char(0)) = 0), protocol TEXT NOT NULL CHECK (protocol IN ('DIRECT','MULTIPART')), expected_size INTEGER NOT NULL CHECK (expected_size >= 0 AND expected_size <= 21474836480), expected_sha256 TEXT CHECK (expected_sha256 IS NULL OR (length(expected_sha256) = 64 AND lower(expected_sha256) NOT GLOB '*[^0-9a-f]*')), actual_size INTEGER CHECK (actual_size IS NULL OR (actual_size >= 0 AND actual_size <= 21474836480)), actual_sha256 TEXT CHECK (actual_sha256 IS NULL OR (length(actual_sha256) = 64 AND lower(actual_sha256) NOT GLOB '*[^0-9a-f]*')), provider_id BLOB, temporary_object_key TEXT NOT NULL CHECK (length(trim(temporary_object_key)) > 0), status TEXT NOT NULL CHECK (status IN ('INITIATED','UPLOADING','PAUSED','VERIFYING','SCANNING','COMMITTING','COMPLETED','FAILED','REJECTED','ABORTED','EXPIRED')), classification TEXT NOT NULL CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED')), committed_node_id BLOB, scan_clean INTEGER, scan_threat_name TEXT, scan_engine TEXT, scan_signature TEXT, scan_completed_at DATETIME, error_code TEXT, idempotency_key TEXT NOT NULL CHECK (length(trim(idempotency_key)) > 0), request_hash TEXT NOT NULL CHECK (length(request_hash) = 64 AND lower(request_hash) NOT GLOB '*[^0-9a-f]*'), expires_at DATETIME NOT NULL, version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0), created_by BLOB NOT NULL, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, UNIQUE(workspace_id, idempotency_key), UNIQUE(namespace_id, idempotency_key), UNIQUE(workspace_id, id), FOREIGN KEY(namespace_id) REFERENCES namespaces(id), FOREIGN KEY(target_parent_id, namespace_id) REFERENCES nodes(id, namespace_id), FOREIGN KEY(committed_node_id) REFERENCES nodes(id))`,
			`CREATE INDEX IF NOT EXISTS filesystem.ix_transfer_sessions_workspace_status ON transfer_sessions(workspace_id, status)`,
			`CREATE INDEX IF NOT EXISTS filesystem.ix_transfer_sessions_workspace_created ON transfer_sessions(workspace_id, created_at DESC, id DESC)`,
			`CREATE INDEX IF NOT EXISTS filesystem.ix_transfer_sessions_workspace_status_created ON transfer_sessions(workspace_id, status, created_at DESC, id DESC)`,
			`CREATE INDEX IF NOT EXISTS filesystem.ix_transfer_sessions_expires_at ON transfer_sessions(expires_at)`,
			`CREATE TABLE IF NOT EXISTS filesystem.transfer_parts (id BLOB PRIMARY KEY, session_id BLOB NOT NULL, part_number INTEGER NOT NULL CHECK (part_number > 0), offset_bytes INTEGER NOT NULL CHECK (offset_bytes >= 0), size_bytes INTEGER NOT NULL CHECK (size_bytes > 0), checksum TEXT CHECK (checksum IS NULL OR (length(checksum) = 64 AND lower(checksum) NOT GLOB '*[^0-9a-f]*')), etag TEXT NOT NULL CHECK (length(trim(etag)) > 0), created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, UNIQUE(session_id, part_number), FOREIGN KEY(session_id) REFERENCES transfer_sessions(id))`,
			`CREATE INDEX IF NOT EXISTS filesystem.ix_transfer_parts_session_offset ON transfer_parts(session_id, offset_bytes)`,
			`CREATE TRIGGER IF NOT EXISTS filesystem.trg_transfer_parts_insert_bounds BEFORE INSERT ON transfer_parts BEGIN SELECT CASE WHEN NEW.offset_bytes > (SELECT expected_size FROM transfer_sessions WHERE id = NEW.session_id) OR NEW.size_bytes > (SELECT expected_size FROM transfer_sessions WHERE id = NEW.session_id) - NEW.offset_bytes THEN RAISE(ABORT, 'transfer part exceeds expected session size') END; END`,
			`CREATE TRIGGER IF NOT EXISTS filesystem.trg_transfer_parts_update_bounds BEFORE UPDATE ON transfer_parts BEGIN SELECT CASE WHEN NEW.offset_bytes > (SELECT expected_size FROM transfer_sessions WHERE id = NEW.session_id) OR NEW.size_bytes > (SELECT expected_size FROM transfer_sessions WHERE id = NEW.session_id) - NEW.offset_bytes THEN RAISE(ABORT, 'transfer part exceeds expected session size') END; END`,
			`CREATE TABLE IF NOT EXISTS filesystem.quota_reservations (id BLOB PRIMARY KEY, quota_account_id BLOB NOT NULL, namespace_id BLOB NOT NULL, workspace_id BLOB, session_id BLOB, bytes INTEGER NOT NULL CHECK (bytes >= 0), files INTEGER NOT NULL CHECK (files >= 0), state TEXT NOT NULL, expires_at DATETIME NOT NULL, created_at DATETIME, FOREIGN KEY(quota_account_id, namespace_id) REFERENCES quota_accounts(id, namespace_id), FOREIGN KEY(session_id) REFERENCES transfer_sessions(id))`,
			`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_quota_reservations_session ON quota_reservations(session_id) WHERE session_id IS NOT NULL`,
			`CREATE TABLE IF NOT EXISTS filesystem.resource_grants (id BLOB PRIMARY KEY, node_id BLOB NOT NULL, organization_id BLOB NOT NULL, grantee_id BLOB NOT NULL, principal_type TEXT NOT NULL CHECK (principal_type IN ('USER','WORKSPACE','ORGANIZATION','ROLE','SERVICE_ACCOUNT','AGENT')), principal_id BLOB NOT NULL, permission TEXT NOT NULL CHECK (permission IN ('READ','DOWNLOAD','WRITE','DELETE','SHARE','ADMIN')), effect TEXT NOT NULL CHECK (effect IN ('ALLOW','DENY')), inherit_to_children INTEGER NOT NULL DEFAULT 1 CHECK (inherit_to_children IN (0, 1)), expires_at DATETIME, revoked_at DATETIME, policy_version INTEGER NOT NULL DEFAULT 1, created_at DATETIME, FOREIGN KEY(node_id) REFERENCES nodes(id))`,
			`CREATE TABLE IF NOT EXISTS filesystem.share_requests (id BLOB PRIMARY KEY, organization_id BLOB NOT NULL, namespace_id BLOB NOT NULL, node_id BLOB NOT NULL, requested_by BLOB NOT NULL, principal_type TEXT NOT NULL CHECK (principal_type IN ('USER','WORKSPACE','ORGANIZATION','ROLE')), principal_ids TEXT NOT NULL, actions TEXT NOT NULL, inherit_to_children INTEGER NOT NULL DEFAULT 1 CHECK (inherit_to_children IN (0, 1)), expires_at DATETIME, classification TEXT NOT NULL CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED')), effect TEXT NOT NULL DEFAULT 'ALLOW' CHECK (effect IN ('ALLOW','DENY')), status TEXT NOT NULL CHECK (status IN ('PENDING','APPROVED','REJECTED','CANCELLED')), reviewed_by BLOB, reviewed_at DATETIME, rejection_reason TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL, FOREIGN KEY(namespace_id) REFERENCES namespaces(id), FOREIGN KEY(node_id) REFERENCES nodes(id))`,
			`CREATE INDEX IF NOT EXISTS filesystem.ix_filesystem_share_requests_org_status ON share_requests(organization_id, status, created_at)`,
			`CREATE INDEX IF NOT EXISTS filesystem.ix_filesystem_share_requests_node ON share_requests(node_id, created_at)`,
			`CREATE TABLE IF NOT EXISTS filesystem.file_links (id BLOB PRIMARY KEY, node_id BLOB NOT NULL, revision_id BLOB, link_type TEXT NOT NULL, link_id BLOB NOT NULL, created_at DATETIME, UNIQUE(link_type, link_id, node_id, revision_id), FOREIGN KEY(node_id) REFERENCES nodes(id), FOREIGN KEY(revision_id) REFERENCES file_revisions(id))`,
			`CREATE TABLE IF NOT EXISTS filesystem.file_lineage (id BLOB PRIMARY KEY, organization_id BLOB NOT NULL, source_namespace_id BLOB NOT NULL, source_node_id BLOB NOT NULL, source_revision_id BLOB NOT NULL, target_namespace_id BLOB NOT NULL, target_node_id BLOB NOT NULL UNIQUE, target_revision_id BLOB NOT NULL, relation TEXT NOT NULL CHECK (relation = 'ADOPTED_AS_ORGANIZATION_FILE'), command_id BLOB NOT NULL UNIQUE, created_by BLOB NOT NULL, created_at DATETIME NOT NULL, CHECK (source_node_id <> target_node_id), FOREIGN KEY(source_namespace_id) REFERENCES namespaces(id), FOREIGN KEY(source_node_id, source_namespace_id) REFERENCES nodes(id, namespace_id), FOREIGN KEY(source_revision_id, source_node_id) REFERENCES file_revisions(id, node_id), FOREIGN KEY(target_namespace_id) REFERENCES namespaces(id), FOREIGN KEY(target_node_id, target_namespace_id) REFERENCES nodes(id, namespace_id), FOREIGN KEY(target_revision_id, target_node_id) REFERENCES file_revisions(id, node_id))`,
			`CREATE INDEX IF NOT EXISTS filesystem.ix_file_lineage_source ON file_lineage(organization_id, source_node_id, created_at DESC)`,
			`CREATE TABLE IF NOT EXISTS filesystem.command_dedup (command_id BLOB PRIMARY KEY, workspace_id BLOB NOT NULL, operation TEXT NOT NULL, request_hash TEXT NOT NULL, node_id BLOB, revision_id BLOB, state TEXT NOT NULL, created_at DATETIME, updated_at DATETIME)`,
			`CREATE TABLE IF NOT EXISTS filesystem.storage_providers (id BLOB PRIMARY KEY, organization_id BLOB NOT NULL, name TEXT NOT NULL CHECK (length(trim(name)) > 0), provider_type TEXT NOT NULL CHECK (provider_type IN ('LOCAL','NFS','S3')), endpoint TEXT NOT NULL DEFAULT '', secret_ref TEXT NOT NULL CHECK (length(trim(secret_ref)) > 0), capabilities TEXT NOT NULL DEFAULT '{}', health TEXT NOT NULL DEFAULT '{}', config_version INTEGER NOT NULL DEFAULT 1 CHECK (config_version > 0), status TEXT NOT NULL DEFAULT 'ACTIVE', created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, UNIQUE(organization_id, name))`,
			`CREATE TABLE IF NOT EXISTS filesystem.capabilities (id BLOB PRIMARY KEY, token_hash TEXT NOT NULL UNIQUE, organization_id BLOB NOT NULL, workspace_id BLOB NOT NULL, actor_id BLOB NOT NULL, agent_id BLOB, run_id BLOB NOT NULL, resource_ids TEXT NOT NULL, actions TEXT NOT NULL, audience TEXT NOT NULL, expires_at DATETIME NOT NULL, revoked_at DATETIME, created_at DATETIME NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS filesystem.export_tasks (id BLOB PRIMARY KEY, source_organization_id BLOB NOT NULL, target_organization_id BLOB NOT NULL, source_workspace_id BLOB NOT NULL, target_workspace_id BLOB NOT NULL, source_node_id BLOB NOT NULL, target_node_id BLOB, status TEXT NOT NULL CHECK (status IN ('PENDING','IMPORTED')), actor_id BLOB NOT NULL, classification TEXT NOT NULL DEFAULT 'RESTRICTED' CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED')), created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL)`,
			`CREATE TABLE IF NOT EXISTS filesystem.artifacts (id BLOB PRIMARY KEY, workspace_id BLOB NOT NULL, namespace_id BLOB NOT NULL, artifact_type TEXT NOT NULL, producer_type TEXT NOT NULL, producer_id BLOB NOT NULL, run_id BLOB NOT NULL, input_refs TEXT NOT NULL DEFAULT '[]', file_id BLOB NOT NULL, revision_id BLOB, retention_policy_id BLOB, created_at DATETIME NOT NULL, FOREIGN KEY(file_id) REFERENCES nodes(id))`,
		} {
			if err := db.Exec(statement).Error; err != nil {
				return wrapPersistence(err)
			}
		}
		if err := upgradeSQLiteResourceGrants(db); err != nil {
			return err
		}
		hadNodeClassification, err := sqliteHasColumn(db, "nodes", "classification")
		if err != nil {
			return err
		}
		for _, statement := range []string{
			`ALTER TABLE filesystem.nodes ADD COLUMN classification TEXT NOT NULL DEFAULT 'RESTRICTED' CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED'))`,
			`ALTER TABLE filesystem.transfer_sessions ADD COLUMN committed_node_id BLOB`,
			`ALTER TABLE filesystem.export_tasks ADD COLUMN classification TEXT NOT NULL DEFAULT 'RESTRICTED' CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED'))`,
			`ALTER TABLE filesystem.share_requests ADD COLUMN effect TEXT NOT NULL DEFAULT 'ALLOW' CHECK (effect IN ('ALLOW','DENY'))`,
			`ALTER TABLE filesystem.blobs ADD COLUMN tombstoned_at DATETIME`,
			`ALTER TABLE filesystem.blobs ADD COLUMN legal_hold INTEGER NOT NULL DEFAULT 0 CHECK (legal_hold IN (0, 1))`,
		} {
			if err := db.Exec(statement).Error; err != nil && !isSQLiteDuplicateColumn(err) {
				return wrapPersistence(err)
			}
		}
		if !hadNodeClassification {
			if err := backfillSQLiteNodeClassifications(db); err != nil {
				return err
			}
		}
		return nil
	}
	postgresNeedsClassificationBackfill := false
	if postgres.ShouldRunAutoMigrate(db) && db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS filesystem").Error; err != nil {
			return wrapPersistence(err)
		}
		hadNodes, err := postgresHasColumn(db, "nodes", "id")
		if err != nil {
			return err
		}
		hadClassification, err := postgresHasColumn(db, "nodes", "classification")
		if err != nil {
			return err
		}
		postgresNeedsClassificationBackfill = hadNodes && !hadClassification
	}
	if err := db.AutoMigrate(&namespaceRow{}, &nodeRow{}, &blobRow{}, &revisionRow{}, &replicaRow{}, &replicaMigrationRow{}, &quotaAccountRow{}, &transferSessionRow{}, &transferPartRow{}, &quotaReservationRow{}, &resourceGrantRow{}, &shareRequestRow{}, &fileLinkRow{}, &fileLineageRow{}, &commandDedupRow{}, &storageProviderRow{}, &storagePoolRow{}, &storagePoolMemberRow{}, &storageBindingRow{}, &lifecyclePolicyRow{}, &capabilityRow{}, &exportTaskRow{}, &artifactRow{}); err != nil {
		return wrapPersistence(err)
	}
	if db.Dialector.Name() == "postgres" {
		if postgresNeedsClassificationBackfill {
			if err := db.Exec(postgresNodeClassificationBackfill).Error; err != nil {
				return wrapPersistence(err)
			}
		}
		for _, statement := range postgresFallbackConstraints {
			if err := db.Exec(statement).Error; err != nil {
				return wrapPersistence(err)
			}
		}
	}
	return nil
}

func postgresHasColumn(db *gorm.DB, table, column string) (bool, error) {
	var exists bool
	err := db.Raw(`
SELECT EXISTS (
	SELECT 1 FROM information_schema.columns
	WHERE table_schema = 'filesystem' AND table_name = ? AND column_name = ?
)`, table, column).Scan(&exists).Error
	if err != nil {
		return false, wrapPersistence(err)
	}
	return exists, nil
}

func sqliteHasColumn(db *gorm.DB, table, column string) (bool, error) {
	var count int64
	if err := db.Raw(fmt.Sprintf(`SELECT COUNT(*) FROM filesystem.pragma_table_info('%s') WHERE name = ?`, table), column).Scan(&count).Error; err != nil {
		return false, wrapPersistence(err)
	}
	return count > 0, nil
}

func backfillSQLiteNodeClassifications(db *gorm.DB) error {
	if err := db.Exec(`UPDATE filesystem.nodes SET classification =
		CASE WHEN node_type = 'DIRECTORY' THEN 'INTERNAL' ELSE 'RESTRICTED' END`).Error; err != nil {
		return wrapPersistence(err)
	}
	var sessions []struct {
		ID              uuid.UUID
		Classification  model.TransferClassification
		CommittedNodeID *uuid.UUID
	}
	if err := db.Table("filesystem.transfer_sessions").
		Select("id, classification, committed_node_id").
		Where("status = ?", model.TransferCompleted).Find(&sessions).Error; err != nil {
		return wrapPersistence(err)
	}
	highest := map[uuid.UUID]model.TransferClassification{}
	for _, session := range sessions {
		nodeID := session.CommittedNodeID
		if nodeID == nil {
			candidate := uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-transfer-node:"+session.ID.String()))
			var count int64
			if err := db.Table("filesystem.nodes").Where("id = ?", candidate).Count(&count).Error; err != nil {
				return wrapPersistence(err)
			}
			if count == 0 {
				continue
			}
			nodeID = &candidate
			if err := db.Table("filesystem.transfer_sessions").Where("id = ?", session.ID).
				Update("committed_node_id", candidate).Error; err != nil {
				return wrapPersistence(err)
			}
		}
		if classificationRank(session.Classification) > classificationRank(highest[*nodeID]) {
			highest[*nodeID] = session.Classification
		}
	}
	for nodeID, classification := range highest {
		if !validClassification(classification) {
			continue
		}
		if err := db.Table("filesystem.nodes").Where("id = ?", nodeID).
			Update("classification", classification).Error; err != nil {
			return wrapPersistence(err)
		}
	}
	return nil
}

func classificationRank(classification model.TransferClassification) int {
	switch classification {
	case model.TransferClassificationPublic:
		return 1
	case model.TransferClassificationInternal:
		return 2
	case model.TransferClassificationConfidential:
		return 3
	case model.TransferClassificationRestricted:
		return 4
	default:
		return 0
	}
}

func upgradeSQLiteResourceGrants(db *gorm.DB) error {
	columns, notNull, err := sqliteResourceGrantColumns(db)
	if err != nil {
		return err
	}
	var tableSQL string
	if err := db.Raw("SELECT sql FROM filesystem.sqlite_master WHERE type = 'table' AND name = 'resource_grants'").Scan(&tableSQL).Error; err != nil {
		return wrapPersistence(err)
	}
	if !columns["organization_id"] || !columns["principal_type"] || !columns["principal_id"] ||
		!notNull["organization_id"] || !notNull["principal_type"] || !notNull["principal_id"] ||
		!strings.Contains(tableSQL, "'DOWNLOAD'") {
		if err := rebuildSQLiteResourceGrants(db, columns); err != nil {
			return err
		}
	} else {
		for _, column := range []struct {
			name string
			sql  string
		}{
			{"organization_id", "ALTER TABLE filesystem.resource_grants ADD COLUMN organization_id BLOB"},
			{"principal_id", "ALTER TABLE filesystem.resource_grants ADD COLUMN principal_id BLOB"},
			{"inherit_to_children", "ALTER TABLE filesystem.resource_grants ADD COLUMN inherit_to_children INTEGER DEFAULT 1"},
			{"revoked_at", "ALTER TABLE filesystem.resource_grants ADD COLUMN revoked_at DATETIME"},
			{"policy_version", "ALTER TABLE filesystem.resource_grants ADD COLUMN policy_version INTEGER DEFAULT 1"},
		} {
			if !columns[column.name] {
				if err := db.Exec(column.sql).Error; err != nil {
					return wrapPersistence(err)
				}
			}
		}
		if err := db.Exec(`UPDATE filesystem.resource_grants
			SET principal_type = COALESCE(NULLIF(principal_type, ''), 'USER'),
				principal_id = COALESCE(principal_id, grantee_id),
				organization_id = COALESCE(organization_id, (SELECT ns.organization_id FROM filesystem.nodes n JOIN filesystem.namespaces ns ON ns.id = n.namespace_id WHERE n.id = filesystem.resource_grants.node_id)),
				inherit_to_children = COALESCE(inherit_to_children, 1),
				policy_version = COALESCE(policy_version, 1)
			WHERE principal_type IS NULL OR principal_type = '' OR principal_id IS NULL OR organization_id IS NULL`).Error; err != nil {
			return wrapPersistence(err)
		}
	}
	if err := db.Exec("DROP INDEX IF EXISTS filesystem.ux_filesystem_grants_identity").Error; err != nil {
		return wrapPersistence(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_grants_active_identity ON resource_grants(node_id, principal_type, principal_id, permission, effect) WHERE revoked_at IS NULL").Error; err != nil {
		return wrapPersistence(err)
	}
	return nil
}

func sqliteResourceGrantColumns(db *gorm.DB) (map[string]bool, map[string]bool, error) {
	var rows []struct {
		Name    string `gorm:"column:name"`
		NotNull int    `gorm:"column:notnull"`
	}
	if err := db.Raw(`SELECT name, "notnull" FROM filesystem.pragma_table_info('resource_grants')`).Scan(&rows).Error; err != nil {
		return nil, nil, wrapPersistence(err)
	}
	columns := make(map[string]bool, len(rows))
	notNull := make(map[string]bool, len(rows))
	for _, row := range rows {
		columns[row.Name] = true
		notNull[row.Name] = row.NotNull == 1
	}
	return columns, notNull, nil
}

func rebuildSQLiteResourceGrants(db *gorm.DB, columns map[string]bool) error {
	if err := db.Exec("ALTER TABLE filesystem.resource_grants RENAME TO resource_grants_legacy").Error; err != nil {
		return wrapPersistence(err)
	}
	if err := db.Exec(`CREATE TABLE filesystem.resource_grants (
		id BLOB PRIMARY KEY,
		node_id BLOB NOT NULL,
		organization_id BLOB NOT NULL,
		grantee_id BLOB NOT NULL,
		principal_type TEXT NOT NULL CHECK (principal_type IN ('USER','WORKSPACE','ORGANIZATION','ROLE','SERVICE_ACCOUNT','AGENT')),
		principal_id BLOB NOT NULL,
		permission TEXT NOT NULL CHECK (permission IN ('READ','DOWNLOAD','WRITE','DELETE','SHARE','ADMIN')),
		effect TEXT NOT NULL CHECK (effect IN ('ALLOW','DENY')),
		inherit_to_children INTEGER NOT NULL DEFAULT 1 CHECK (inherit_to_children IN (0, 1)),
		expires_at DATETIME,
		revoked_at DATETIME,
		policy_version INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME,
		FOREIGN KEY(node_id) REFERENCES nodes(id)
	)`).Error; err != nil {
		return wrapPersistence(err)
	}
	organization := "(SELECT ns.organization_id FROM filesystem.nodes n JOIN filesystem.namespaces ns ON ns.id = n.namespace_id WHERE n.id = old.node_id)"
	principalType, principalID := "'USER'", "old.grantee_id"
	inherit, expires, revoked, policy := "1", "NULL", "NULL", "1"
	if columns["principal_type"] {
		principalType = "COALESCE(NULLIF(old.principal_type, ''), 'USER')"
	}
	if columns["principal_id"] {
		principalID = "COALESCE(old.principal_id, old.grantee_id)"
	}
	if columns["organization_id"] {
		organization = "COALESCE(old.organization_id, " + organization + ")"
	}
	if columns["inherit_to_children"] {
		inherit = "COALESCE(old.inherit_to_children, 1)"
	}
	if columns["expires_at"] {
		expires = "old.expires_at"
	}
	if columns["revoked_at"] {
		revoked = "old.revoked_at"
	}
	if columns["policy_version"] {
		policy = "COALESCE(old.policy_version, 1)"
	}
	query := fmt.Sprintf(`INSERT INTO filesystem.resource_grants
		(id, node_id, organization_id, grantee_id, principal_type, principal_id,
		 permission, effect, inherit_to_children, expires_at, revoked_at, policy_version, created_at)
		SELECT old.id, old.node_id, %s, old.grantee_id, %s, %s, old.permission, old.effect,
			%s, %s, %s, %s, old.created_at
		FROM filesystem.resource_grants_legacy old`, organization, principalType, principalID, inherit, expires, revoked, policy)
	if err := db.Exec(query).Error; err != nil {
		return wrapPersistence(err)
	}
	if err := db.Exec("DROP TABLE filesystem.resource_grants_legacy").Error; err != nil {
		return wrapPersistence(err)
	}
	return nil
}

func isSQLiteDuplicateColumn(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate column") || strings.Contains(message, "duplicate column name")
}

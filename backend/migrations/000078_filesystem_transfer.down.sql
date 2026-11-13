DROP INDEX IF EXISTS filesystem.ux_quota_reservations_session;

ALTER TABLE filesystem.quota_reservations
    DROP CONSTRAINT IF EXISTS fk_quota_reservations_account_workspace;
ALTER TABLE filesystem.quota_reservations
    DROP CONSTRAINT IF EXISTS fk_quota_reservations_session;

ALTER TABLE filesystem.quota_reservations
    DROP COLUMN session_id,
    DROP COLUMN workspace_id;

ALTER TABLE filesystem.quota_accounts
    DROP CONSTRAINT IF EXISTS uq_quota_accounts_id_workspace;

DROP INDEX IF EXISTS filesystem.ix_transfer_parts_session_offset;
DROP INDEX IF EXISTS filesystem.ix_transfer_sessions_expires_at;
DROP INDEX IF EXISTS filesystem.ix_transfer_sessions_workspace_status;

DROP TRIGGER IF EXISTS trg_transfer_parts_validate_bounds
    ON filesystem.transfer_parts;
DROP FUNCTION IF EXISTS filesystem.validate_transfer_part_bounds();

ALTER TABLE IF EXISTS filesystem.transfer_parts
    DROP CONSTRAINT IF EXISTS fk_transfer_parts_session;
ALTER TABLE IF EXISTS filesystem.transfer_sessions
    DROP CONSTRAINT IF EXISTS fk_transfer_sessions_parent_namespace;
ALTER TABLE IF EXISTS filesystem.transfer_sessions
    DROP CONSTRAINT IF EXISTS fk_transfer_sessions_namespace_workspace;

DROP TABLE IF EXISTS filesystem.transfer_parts;
DROP TABLE IF EXISTS filesystem.transfer_sessions;

ALTER TABLE filesystem.namespaces
    DROP CONSTRAINT IF EXISTS fk_namespaces_workspace_id;

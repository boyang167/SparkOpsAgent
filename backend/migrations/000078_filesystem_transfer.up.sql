ALTER TABLE filesystem.namespaces
    ADD CONSTRAINT fk_namespaces_workspace_id UNIQUE (workspace_id, id);

CREATE TABLE filesystem.transfer_sessions (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    namespace_id UUID NOT NULL,
    target_parent_id UUID,
    target_name TEXT NOT NULL CHECK (length(btrim(target_name)) > 0 AND target_name !~ '[/\\\x00]'),
    protocol VARCHAR(24) NOT NULL CHECK (protocol IN ('DIRECT','MULTIPART')),
    expected_size BIGINT NOT NULL CHECK (expected_size >= 0 AND expected_size <= 21474836480),
    expected_sha256 CHAR(64) CHECK (expected_sha256 IS NULL OR expected_sha256 ~ '^[0-9A-Fa-f]{64}$'),
    actual_size BIGINT CHECK (actual_size IS NULL OR (actual_size >= 0 AND actual_size <= 21474836480)),
    actual_sha256 CHAR(64) CHECK (actual_sha256 IS NULL OR actual_sha256 ~ '^[0-9A-Fa-f]{64}$'),
    provider_id UUID,
    temporary_object_key TEXT NOT NULL CHECK (length(btrim(temporary_object_key)) > 0),
    status VARCHAR(24) NOT NULL CHECK (status IN ('INITIATED','UPLOADING','PAUSED','VERIFYING','SCANNING','COMMITTING','COMPLETED','FAILED','REJECTED','ABORTED','EXPIRED')),
    classification VARCHAR(16) NOT NULL CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED')),
    scan_clean BOOLEAN,
    scan_threat_name TEXT,
    scan_engine TEXT,
    scan_signature TEXT,
    error_code TEXT,
    idempotency_key TEXT NOT NULL CHECK (length(btrim(idempotency_key)) > 0),
    expires_at TIMESTAMPTZ NOT NULL,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, idempotency_key),
    UNIQUE (workspace_id, id),
    CONSTRAINT fk_transfer_sessions_namespace_workspace
        FOREIGN KEY (workspace_id, namespace_id)
        REFERENCES filesystem.namespaces(workspace_id, id),
    CONSTRAINT fk_transfer_sessions_parent_namespace
        FOREIGN KEY (target_parent_id, namespace_id)
        REFERENCES filesystem.nodes(id, namespace_id)
);

CREATE INDEX ix_transfer_sessions_workspace_status
    ON filesystem.transfer_sessions(workspace_id, status);
CREATE INDEX ix_transfer_sessions_expires_at
    ON filesystem.transfer_sessions(expires_at);

CREATE TABLE filesystem.transfer_parts (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL,
    part_number INT NOT NULL CHECK (part_number > 0),
    offset_bytes BIGINT NOT NULL,
    size_bytes BIGINT NOT NULL,
    checksum CHAR(64) CHECK (checksum IS NULL OR checksum ~ '^[0-9A-Fa-f]{64}$'),
    etag TEXT NOT NULL CHECK (length(btrim(etag)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (offset_bytes >= 0 AND size_bytes > 0),
    UNIQUE (session_id, part_number),
    CONSTRAINT fk_transfer_parts_session
        FOREIGN KEY (session_id) REFERENCES filesystem.transfer_sessions(id)
);

CREATE INDEX ix_transfer_parts_session_offset
    ON filesystem.transfer_parts(session_id, offset_bytes);

CREATE OR REPLACE FUNCTION filesystem.validate_transfer_part_bounds()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    session_row filesystem.transfer_sessions%ROWTYPE;
BEGIN
    SELECT *
    INTO session_row
    FROM filesystem.transfer_sessions
    WHERE id = NEW.session_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'transfer session % does not exist', NEW.session_id
            USING ERRCODE = '23503';
    END IF;

    IF NEW.offset_bytes > session_row.expected_size
       OR NEW.size_bytes > session_row.expected_size - NEW.offset_bytes THEN
        RAISE EXCEPTION 'transfer part exceeds expected session size'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_transfer_parts_validate_bounds
BEFORE INSERT OR UPDATE ON filesystem.transfer_parts
FOR EACH ROW
EXECUTE FUNCTION filesystem.validate_transfer_part_bounds();

ALTER TABLE filesystem.quota_reservations
    ADD COLUMN workspace_id UUID NULL,
    ADD COLUMN session_id UUID NULL;

UPDATE filesystem.quota_reservations reservation
SET workspace_id = account.workspace_id
FROM filesystem.quota_accounts account
WHERE reservation.quota_account_id = account.id;

ALTER TABLE filesystem.quota_reservations
    ALTER COLUMN workspace_id SET NOT NULL;

ALTER TABLE filesystem.quota_accounts
    ADD CONSTRAINT uq_quota_accounts_id_workspace UNIQUE (id, workspace_id);

ALTER TABLE filesystem.quota_reservations
    ADD CONSTRAINT fk_quota_reservations_session
    FOREIGN KEY (workspace_id, session_id)
    REFERENCES filesystem.transfer_sessions(workspace_id, id);

ALTER TABLE filesystem.quota_reservations
    ADD CONSTRAINT fk_quota_reservations_account_workspace
    FOREIGN KEY (quota_account_id, workspace_id)
    REFERENCES filesystem.quota_accounts(id, workspace_id);

CREATE UNIQUE INDEX ux_quota_reservations_session
    ON filesystem.quota_reservations(workspace_id, session_id)
    WHERE session_id IS NOT NULL;

ALTER TABLE knowledge.documents
    ADD COLUMN IF NOT EXISTS file_id UUID,
    ADD COLUMN IF NOT EXISTS migration_status VARCHAR(32);

CREATE TABLE IF NOT EXISTS knowledge.document_file_migrations (
    document_id   UUID PRIMARY KEY,
    file_id       UUID,
    source_sha256 VARCHAR(64) NOT NULL,
    target_sha256 VARCHAR(64),
    status        VARCHAR(32) NOT NULL,
    error_code    VARCHAR(64),
    verified_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_document_file_migrations_status
    ON knowledge.document_file_migrations (status);

CREATE INDEX IF NOT EXISTS idx_documents_migration_status
    ON knowledge.documents (migration_status);

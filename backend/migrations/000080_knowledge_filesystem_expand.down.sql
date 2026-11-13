DROP INDEX IF EXISTS knowledge.idx_documents_migration_status;
DROP INDEX IF EXISTS knowledge.idx_document_file_migrations_status;

DROP TABLE IF EXISTS knowledge.document_file_migrations;

ALTER TABLE knowledge.documents
    DROP COLUMN IF EXISTS migration_status,
    DROP COLUMN IF EXISTS file_id;

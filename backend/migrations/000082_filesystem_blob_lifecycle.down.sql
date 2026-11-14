ALTER TABLE filesystem.storage_replicas
    DROP COLUMN IF EXISTS delete_after;

ALTER TABLE filesystem.blobs
    DROP COLUMN IF EXISTS legal_hold,
    DROP COLUMN IF EXISTS tombstoned_at,
    DROP COLUMN IF EXISTS preferred_provider_id;

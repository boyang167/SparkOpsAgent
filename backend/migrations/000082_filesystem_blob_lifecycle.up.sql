ALTER TABLE filesystem.blobs
    ADD COLUMN IF NOT EXISTS preferred_provider_id UUID REFERENCES filesystem.storage_providers(id),
    ADD COLUMN IF NOT EXISTS tombstoned_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS legal_hold BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE filesystem.storage_replicas
    ADD COLUMN IF NOT EXISTS delete_after TIMESTAMPTZ;

ALTER TABLE delivery.deployment_runs
    ADD COLUMN IF NOT EXISTS sync_manifests JSONB NOT NULL DEFAULT '[]'::jsonb;

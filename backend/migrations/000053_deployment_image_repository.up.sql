ALTER TABLE delivery.deployment_runs
    ADD COLUMN IF NOT EXISTS image_repository VARCHAR(512) NOT NULL DEFAULT '';

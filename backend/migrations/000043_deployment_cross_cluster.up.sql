ALTER TABLE delivery.deployment_runs
    ADD COLUMN IF NOT EXISTS cross_cluster BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE delivery.deployment_runs
    ADD COLUMN IF NOT EXISTS run_type VARCHAR(16) NOT NULL DEFAULT 'normal',
    ADD COLUMN IF NOT EXISTS rollback_of_run_id VARCHAR(36) NULL;

CREATE INDEX IF NOT EXISTS idx_deployment_runs_rollback_of
    ON delivery.deployment_runs (rollback_of_run_id);

CREATE TABLE IF NOT EXISTS delivery.environment_sync_snapshots (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    product_id UUID NOT NULL,
    workflow_id UUID NOT NULL,
    job_name VARCHAR(255) NOT NULL,
    source_environment_id UUID NOT NULL,
    target_environment_id UUID NOT NULL,
    projects JSONB NOT NULL,
    checksum VARCHAR(64) NOT NULL,
    created_by UUID NOT NULL,
    bound_run_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    bound_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_environment_sync_snapshots_product_created
    ON delivery.environment_sync_snapshots(product_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_environment_sync_snapshots_created_by
    ON delivery.environment_sync_snapshots(created_by, created_at DESC);

ALTER TABLE delivery.deployment_runs
    ADD COLUMN IF NOT EXISTS source_environment_id UUID,
    ADD COLUMN IF NOT EXISTS environment_sync_snapshot_id UUID;

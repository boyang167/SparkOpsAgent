ALTER TABLE delivery.deployment_runs
    DROP COLUMN IF EXISTS environment_sync_snapshot_id,
    DROP COLUMN IF EXISTS source_environment_id;

DROP INDEX IF EXISTS delivery.idx_environment_sync_snapshots_created_by;
DROP INDEX IF EXISTS delivery.idx_environment_sync_snapshots_product_created;

DROP TABLE IF EXISTS delivery.environment_sync_snapshots;

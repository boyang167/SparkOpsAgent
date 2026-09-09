DROP INDEX IF EXISTS delivery.idx_deployment_runs_rollback_of;

ALTER TABLE delivery.deployment_runs
    DROP COLUMN IF EXISTS rollback_of_run_id,
    DROP COLUMN IF EXISTS run_type;

DROP INDEX IF EXISTS delivery.idx_deployment_runs_environment;
ALTER TABLE delivery.deployment_runs
    DROP COLUMN IF EXISTS deploy_namespace,
    DROP COLUMN IF EXISTS environment_id;

DROP TABLE IF EXISTS product.environment_projects;
DROP TABLE IF EXISTS product.environments;

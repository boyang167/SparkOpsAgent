ALTER TABLE delivery.deployment_runs
    ADD COLUMN IF NOT EXISTS undeploy_options JSONB NULL;

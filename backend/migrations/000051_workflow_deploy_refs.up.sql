-- Product v2 Phase W1b/W2: workflow deploy refs

CREATE TABLE IF NOT EXISTS product.workflow_deploy_refs (
    deployment_run_id UUID PRIMARY KEY,
    run_id            UUID NOT NULL REFERENCES product.product_workflow_runs(id) ON DELETE CASCADE,
    job_name          VARCHAR(255) NOT NULL,
    project_id        UUID NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workflow_deploy_refs_run ON product.workflow_deploy_refs(run_id);

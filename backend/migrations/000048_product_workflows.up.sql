-- Product v2 Phase W1a: product workflow instances and runs

CREATE TABLE IF NOT EXISTS product.product_workflows (
    id               UUID PRIMARY KEY,
    product_id       UUID NOT NULL REFERENCES product.products(id),
    workspace_id     UUID NOT NULL,
    name             VARCHAR(255) NOT NULL,
    key              VARCHAR(255) NOT NULL,
    description      TEXT,
    status           VARCHAR(16) NOT NULL DEFAULT 'draft',
    stages           JSONB NOT NULL DEFAULT '[]',
    template_binding JSONB,
    origin           JSONB NOT NULL DEFAULT '{"kind":"custom"}',
    created_by       UUID NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_product_workflows_product_name ON product.product_workflows(product_id, name) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS product.product_workflow_runs (
    id            UUID PRIMARY KEY,
    workflow_id   UUID NOT NULL REFERENCES product.product_workflows(id),
    product_id    UUID NOT NULL,
    status        VARCHAR(16) NOT NULL DEFAULT 'pending',
    trigger       JSONB NOT NULL DEFAULT '{}',
    input         JSONB NOT NULL DEFAULT '{}',
    stage_runs    JSONB NOT NULL DEFAULT '[]',
    current_stage INT NOT NULL DEFAULT 0,
    error_message TEXT,
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_product_workflow_runs_workflow ON product.product_workflow_runs(workflow_id);

CREATE TABLE IF NOT EXISTS product.workflow_build_refs (
    build_id    UUID PRIMARY KEY,
    run_id      UUID NOT NULL REFERENCES product.product_workflow_runs(id) ON DELETE CASCADE,
    job_name    VARCHAR(255) NOT NULL,
    project_id  UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workflow_build_refs_run ON product.workflow_build_refs(run_id);

ALTER TABLE product.products
    ADD COLUMN IF NOT EXISTS default_workflow_id UUID REFERENCES product.product_workflows(id);

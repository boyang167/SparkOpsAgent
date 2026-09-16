-- Product v2 Phase W0: workflow templates

CREATE TABLE IF NOT EXISTS product.workflow_templates (
    id            UUID PRIMARY KEY,
    scope         VARCHAR(16) NOT NULL,
    workspace_id  UUID,
    name          VARCHAR(255) NOT NULL,
    key           VARCHAR(255) NOT NULL,
    description   TEXT,
    category      VARCHAR(32),
    product_types JSONB NOT NULL DEFAULT '[]',
    status        VARCHAR(16) NOT NULL DEFAULT 'active',
    version       VARCHAR(32) NOT NULL DEFAULT '1.0.0',
    definition    JSONB NOT NULL DEFAULT '{}',
    slot_schema   JSONB NOT NULL DEFAULT '{}',
    key_vals      JSONB NOT NULL DEFAULT '[]',
    runtime       JSONB NOT NULL DEFAULT '{}',
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_workflow_templates_key ON product.workflow_templates(key) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workflow_templates_scope ON product.workflow_templates(scope);
CREATE INDEX IF NOT EXISTS idx_workflow_templates_workspace ON product.workflow_templates(workspace_id);

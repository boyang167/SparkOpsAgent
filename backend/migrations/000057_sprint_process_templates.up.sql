ALTER TABLE product.sprint_processes
    ADD COLUMN IF NOT EXISTS template_key VARCHAR(64),
    ADD COLUMN IF NOT EXISTS description TEXT;

CREATE TABLE IF NOT EXISTS product.workspace_sprint_process_templates (
    id           UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    name         VARCHAR(255) NOT NULL,
    template_key VARCHAR(64),
    description  TEXT,
    stages       JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by   UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ws_sprint_process_tpl
    ON product.workspace_sprint_process_templates(workspace_id);

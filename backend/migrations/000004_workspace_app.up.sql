CREATE SCHEMA IF NOT EXISTS workspace_app;

CREATE TABLE IF NOT EXISTS workspace_app.projects (
    id           UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    name         VARCHAR(255) NOT NULL,
    description  TEXT,
    status       VARCHAR(32) NOT NULL DEFAULT 'active',
    created_by   UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, name)
);

CREATE INDEX IF NOT EXISTS idx_projects_workspace ON workspace_app.projects(workspace_id);

CREATE TABLE IF NOT EXISTS workspace_app.project_versions (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    project_id UUID NOT NULL,
    product_id UUID,
    version_tag VARCHAR(64) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'ready',
    source VARCHAR(16) NOT NULL DEFAULT 'workflow',
    workflow_run_id UUID,
    workflow_job_name VARCHAR(128) NOT NULL DEFAULT '',
    git_ref_type VARCHAR(16) NOT NULL DEFAULT 'branch',
    git_ref_value VARCHAR(255) NOT NULL DEFAULT '',
    manifests JSONB NOT NULL DEFAULT '[]',
    images JSONB NOT NULL DEFAULT '[]',
    error_message TEXT NOT NULL DEFAULT '',
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_project_versions_project_tag UNIQUE (project_id, version_tag)
);

CREATE INDEX IF NOT EXISTS idx_project_versions_project_created
    ON workspace_app.project_versions (project_id, created_at DESC);

ALTER TABLE delivery.deployment_runs
    ADD COLUMN IF NOT EXISTS project_version_id UUID;

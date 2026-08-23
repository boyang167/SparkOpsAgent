CREATE TABLE IF NOT EXISTS platform_admin.sync_jobs (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    workspace_id UUID NOT NULL,
    version VARCHAR(64),
    source_cluster VARCHAR(255),
    target_cluster VARCHAR(255),
    sync_resources TEXT,
    status VARCHAR(32) DEFAULT 'pending',
    result TEXT,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sync_jobs_workspace ON platform_admin.sync_jobs(workspace_id);

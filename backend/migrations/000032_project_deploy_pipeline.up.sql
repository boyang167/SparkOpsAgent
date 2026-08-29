ALTER TABLE workspace_app.projects
    ADD COLUMN IF NOT EXISTS default_cluster_id UUID,
    ADD COLUMN IF NOT EXISTS jenkins_secret_id UUID,
    ADD COLUMN IF NOT EXISTS kubernetes_secret_id UUID;

ALTER TABLE delivery.builds
    ADD COLUMN IF NOT EXISTS execution_started_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS platform_admin.workspace_clusters (
    workspace_id UUID NOT NULL,
    cluster_id   UUID NOT NULL,
    created_by   UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_workspace_clusters_cluster_info
        FOREIGN KEY (cluster_id)
        REFERENCES platform_admin.cluster_info(id) ON DELETE CASCADE,
    PRIMARY KEY (workspace_id, cluster_id)
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint c
        JOIN pg_class rel ON rel.oid = c.conrelid
        JOIN pg_namespace ns ON ns.oid = rel.relnamespace
        WHERE c.conname = 'fk_workspace_clusters_cluster_info'
          AND c.conrelid = 'platform_admin.workspace_clusters'::regclass
          AND ns.nspname = 'platform_admin'
          AND rel.relname = 'workspace_clusters'
    ) THEN
        ALTER TABLE platform_admin.workspace_clusters
            ADD CONSTRAINT fk_workspace_clusters_cluster_info
            FOREIGN KEY (cluster_id)
            REFERENCES platform_admin.cluster_info(id) ON DELETE CASCADE;
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_workspace_clusters_cluster
    ON platform_admin.workspace_clusters(cluster_id);

ALTER TABLE workspace_app.projects
    DROP CONSTRAINT IF EXISTS fk_projects_default_cluster_workspace;

ALTER TABLE workspace_app.projects
    ADD CONSTRAINT fk_projects_default_cluster_workspace
    FOREIGN KEY (workspace_id, default_cluster_id)
    REFERENCES platform_admin.workspace_clusters(workspace_id, cluster_id);

CREATE TABLE IF NOT EXISTS delivery.deployment_runs (
    id            UUID PRIMARY KEY,
    workspace_id  UUID NOT NULL,
    project_id    UUID NOT NULL,
    cluster_id    UUID NOT NULL,
    git_ref_type  VARCHAR(16) NOT NULL,
    git_ref_value VARCHAR(255) NOT NULL,
    image_tag     VARCHAR(255) NOT NULL,
    status        VARCHAR(16) NOT NULL,
    build_id      UUID,
    error_message TEXT NOT NULL DEFAULT '',
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL,
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ,
    current_stage VARCHAR(16) NOT NULL DEFAULT '',
    version       BIGINT NOT NULL DEFAULT 0,
    attempt_count INT NOT NULL DEFAULT 0,
    lease_owner   VARCHAR(255),
    lease_expires_at TIMESTAMPTZ
);

ALTER TABLE delivery.deployment_runs
    DROP CONSTRAINT IF EXISTS fk_deployment_runs_workspace_cluster;

ALTER TABLE delivery.deployment_runs
    ADD CONSTRAINT fk_deployment_runs_workspace_cluster
    FOREIGN KEY (workspace_id, cluster_id)
    REFERENCES platform_admin.workspace_clusters(workspace_id, cluster_id);

ALTER TABLE delivery.deployment_runs
    DROP CONSTRAINT IF EXISTS chk_deployment_run_status;

ALTER TABLE delivery.deployment_runs
    ADD CONSTRAINT chk_deployment_run_status
    CHECK (status IN ('queued', 'preparing', 'building', 'deploying', 'succeeded', 'failed', 'aborted'));

CREATE UNIQUE INDEX IF NOT EXISTS uq_deployment_runs_active_project
    ON delivery.deployment_runs(project_id)
    WHERE status IN ('queued', 'preparing', 'building', 'deploying');

CREATE TABLE IF NOT EXISTS delivery.deployment_run_events (
    id      BIGSERIAL PRIMARY KEY,
    run_id  UUID NOT NULL,
    at      TIMESTAMPTZ NOT NULL,
    status  VARCHAR(16) NOT NULL,
    stage   VARCHAR(16) NOT NULL,
    message TEXT NOT NULL,
    CONSTRAINT fk_deployment_run_events_run
        FOREIGN KEY (run_id)
        REFERENCES delivery.deployment_runs(id) ON DELETE CASCADE
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint c
        JOIN pg_class rel ON rel.oid = c.conrelid
        JOIN pg_namespace ns ON ns.oid = rel.relnamespace
        WHERE c.conname = 'fk_deployment_run_events_run'
          AND c.conrelid = 'delivery.deployment_run_events'::regclass
          AND ns.nspname = 'delivery'
          AND rel.relname = 'deployment_run_events'
    ) THEN
        ALTER TABLE delivery.deployment_run_events
            ADD CONSTRAINT fk_deployment_run_events_run
            FOREIGN KEY (run_id)
            REFERENCES delivery.deployment_runs(id) ON DELETE CASCADE;
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_deployment_run_events_run_at
    ON delivery.deployment_run_events(run_id, at);

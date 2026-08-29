ALTER TABLE workspace_app.projects
    DROP CONSTRAINT IF EXISTS fk_projects_default_cluster_workspace;

ALTER TABLE delivery.deployment_run_events
    DROP CONSTRAINT IF EXISTS fk_deployment_run_events_run;

DROP TABLE IF EXISTS delivery.deployment_run_events;

ALTER TABLE delivery.deployment_runs
    DROP CONSTRAINT IF EXISTS fk_deployment_runs_workspace_cluster;

DROP TABLE IF EXISTS delivery.deployment_runs;

ALTER TABLE platform_admin.workspace_clusters
    DROP CONSTRAINT IF EXISTS fk_workspace_clusters_cluster_info;

DROP TABLE IF EXISTS platform_admin.workspace_clusters;

ALTER TABLE delivery.builds
    DROP COLUMN IF EXISTS execution_started_at;

ALTER TABLE workspace_app.projects
    DROP COLUMN IF EXISTS kubernetes_secret_id,
    DROP COLUMN IF EXISTS jenkins_secret_id,
    DROP COLUMN IF EXISTS default_cluster_id;

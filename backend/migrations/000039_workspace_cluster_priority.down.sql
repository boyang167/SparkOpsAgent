DROP INDEX IF EXISTS platform_admin.idx_workspace_clusters_workspace_priority;

ALTER TABLE platform_admin.workspace_clusters
    DROP COLUMN IF EXISTS priority_id;

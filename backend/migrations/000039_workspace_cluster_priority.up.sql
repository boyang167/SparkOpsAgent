ALTER TABLE platform_admin.workspace_clusters
    ADD COLUMN IF NOT EXISTS priority_id INT NOT NULL DEFAULT 0;

WITH ranked AS (
    SELECT
        workspace_id,
        cluster_id,
        ROW_NUMBER() OVER (PARTITION BY workspace_id ORDER BY created_at ASC, cluster_id ASC) - 1 AS prio
    FROM platform_admin.workspace_clusters
)
UPDATE platform_admin.workspace_clusters AS wc
SET priority_id = ranked.prio
FROM ranked
WHERE wc.workspace_id = ranked.workspace_id
  AND wc.cluster_id = ranked.cluster_id;

CREATE INDEX IF NOT EXISTS idx_workspace_clusters_workspace_priority
    ON platform_admin.workspace_clusters(workspace_id, priority_id);

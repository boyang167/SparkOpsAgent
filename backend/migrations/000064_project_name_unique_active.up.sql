ALTER TABLE workspace_app.projects
    DROP CONSTRAINT IF EXISTS projects_workspace_id_name_key;

CREATE UNIQUE INDEX IF NOT EXISTS uq_projects_workspace_name_live
    ON workspace_app.projects(workspace_id, name)
    WHERE status <> 'archived';

DROP INDEX IF EXISTS workspace_app.uq_projects_workspace_name_live;

ALTER TABLE workspace_app.projects
    ADD CONSTRAINT projects_workspace_id_name_key UNIQUE (workspace_id, name);

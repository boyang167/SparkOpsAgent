ALTER TABLE workspace_app.projects
    DROP CONSTRAINT IF EXISTS chk_projects_archive_phase;

ALTER TABLE workspace_app.projects
    DROP COLUMN IF EXISTS archive_phase;

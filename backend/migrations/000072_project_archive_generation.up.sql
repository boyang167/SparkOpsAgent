ALTER TABLE workspace_app.projects
    ADD COLUMN IF NOT EXISTS archive_generation UUID;

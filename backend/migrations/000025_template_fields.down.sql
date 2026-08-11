ALTER TABLE workspace_app.templates
    DROP COLUMN IF EXISTS is_public,
    DROP COLUMN IF EXISTS status;

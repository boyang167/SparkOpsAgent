DROP INDEX IF EXISTS workspace_app.idx_projects_language;
DROP INDEX IF EXISTS workspace_app.idx_projects_namespace;

ALTER TABLE workspace_app.projects
    DROP COLUMN IF EXISTS resources,
    DROP COLUMN IF EXISTS template_id,
    DROP COLUMN IF EXISTS dockerfile_path,
    DROP COLUMN IF EXISTS git_ref_value,
    DROP COLUMN IF EXISTS git_ref_type,
    DROP COLUMN IF EXISTS secret_id,
    DROP COLUMN IF EXISTS git_url,
    DROP COLUMN IF EXISTS language,
    DROP COLUMN IF EXISTS namespace,
    DROP COLUMN IF EXISTS display_name;

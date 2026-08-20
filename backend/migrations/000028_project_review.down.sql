DROP INDEX IF EXISTS workspace_app.idx_projects_review_status;

ALTER TABLE workspace_app.projects
    DROP CONSTRAINT IF EXISTS chk_projects_review_status,
    DROP COLUMN IF EXISTS review_comment,
    DROP COLUMN IF EXISTS reviewed_at,
    DROP COLUMN IF EXISTS reviewed_by,
    DROP COLUMN IF EXISTS submitted_at,
    DROP COLUMN IF EXISTS submitted_by,
    DROP COLUMN IF EXISTS review_status;

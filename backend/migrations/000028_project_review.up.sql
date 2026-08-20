ALTER TABLE workspace_app.projects
    ADD COLUMN IF NOT EXISTS review_status VARCHAR(16) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS submitted_by UUID,
    ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS reviewed_by UUID,
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS review_comment TEXT NOT NULL DEFAULT '';

ALTER TABLE workspace_app.projects
    DROP CONSTRAINT IF EXISTS chk_projects_review_status;

ALTER TABLE workspace_app.projects
    ADD CONSTRAINT chk_projects_review_status
    CHECK (review_status IN ('none', 'pending', 'approved', 'rejected'));

CREATE INDEX IF NOT EXISTS idx_projects_review_status
    ON workspace_app.projects(workspace_id, review_status);

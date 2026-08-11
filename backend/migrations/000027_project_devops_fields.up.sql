ALTER TABLE workspace_app.projects
    ADD COLUMN IF NOT EXISTS display_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS namespace VARCHAR(63),
    ADD COLUMN IF NOT EXISTS language VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS git_url TEXT,
    ADD COLUMN IF NOT EXISTS secret_id UUID,
    ADD COLUMN IF NOT EXISTS git_ref_type VARCHAR(16),
    ADD COLUMN IF NOT EXISTS git_ref_value VARCHAR(255),
    ADD COLUMN IF NOT EXISTS dockerfile_path VARCHAR(512) NOT NULL DEFAULT './Dockerfile',
    ADD COLUMN IF NOT EXISTS template_id UUID,
    ADD COLUMN IF NOT EXISTS resources JSONB NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_projects_namespace ON workspace_app.projects(namespace);
CREATE INDEX IF NOT EXISTS idx_projects_language ON workspace_app.projects(language);

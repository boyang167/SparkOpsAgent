ALTER TABLE workspace_app.projects
    ADD COLUMN IF NOT EXISTS archive_phase VARCHAR(32) NOT NULL DEFAULT 'none';

UPDATE workspace_app.projects
SET archive_phase = 'pending'
WHERE status = 'archiving'
  AND archive_phase = 'none';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_projects_archive_phase'
    ) THEN
        ALTER TABLE workspace_app.projects
            ADD CONSTRAINT chk_projects_archive_phase
            CHECK (archive_phase IN (
                'none',
                'pending',
                'undeploy_claimed',
                'undeploying',
                'pipeline_pending',
                'pipeline_claimed',
                'finalizing'
            ));
    END IF;
END $$;

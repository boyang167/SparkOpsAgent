ALTER TABLE workspace_app.projects
    ADD COLUMN IF NOT EXISTS archive_allow_undeploy BOOLEAN;

UPDATE workspace_app.projects
SET archive_allow_undeploy = CASE
    WHEN archive_phase = 'pending' THEN FALSE
    ELSE TRUE
END
WHERE status = 'archiving'
  AND archive_allow_undeploy IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_projects_archive_intent'
          AND conrelid = 'workspace_app.projects'::regclass
    ) THEN
        ALTER TABLE workspace_app.projects
            ADD CONSTRAINT chk_projects_archive_intent
            CHECK (
                (status = 'archiving' AND archive_allow_undeploy IS NOT NULL)
                OR
                (status <> 'archiving' AND archive_allow_undeploy IS NULL)
            );
    END IF;
END $$;

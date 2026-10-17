ALTER TABLE delivery.deployment_runs
    ADD COLUMN IF NOT EXISTS archive_generation UUID;

-- Also covers databases that applied 000072 before its legacy backfill was added.
UPDATE workspace_app.projects
SET archive_generation = (
    substr(md5('workspace-archive:' || id::text), 1, 8) || '-' ||
    substr(md5('workspace-archive:' || id::text), 9, 4) || '-5' ||
    substr(md5('workspace-archive:' || id::text), 14, 3) || '-a' ||
    substr(md5('workspace-archive:' || id::text), 18, 3) || '-' ||
    substr(md5('workspace-archive:' || id::text), 21, 12)
)::uuid
WHERE status = 'archiving'
  AND archive_generation IS NULL;

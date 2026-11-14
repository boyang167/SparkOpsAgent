DROP INDEX IF EXISTS filesystem.ux_filesystem_namespaces_user_owner;
DROP INDEX IF EXISTS filesystem.ux_filesystem_namespaces_workspace_id;
DROP INDEX IF EXISTS filesystem.ux_filesystem_namespaces_organization_id;

ALTER TABLE filesystem.namespaces
    DROP CONSTRAINT IF EXISTS ck_filesystem_namespace_scope,
    DROP CONSTRAINT IF EXISTS ck_filesystem_namespace_type,
    ALTER COLUMN workspace_id SET NOT NULL,
    ADD CONSTRAINT namespaces_workspace_id_key UNIQUE (workspace_id);

ALTER TABLE filesystem.namespaces
    DROP COLUMN IF EXISTS policy_version,
    DROP COLUMN IF EXISTS owner_user_id,
    DROP COLUMN IF EXISTS namespace_type;

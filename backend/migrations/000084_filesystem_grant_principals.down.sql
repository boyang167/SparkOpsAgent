DROP INDEX IF EXISTS filesystem.ux_filesystem_grants_active_identity;

ALTER TABLE filesystem.resource_grants
    DROP COLUMN IF EXISTS policy_version,
    DROP COLUMN IF EXISTS revoked_at,
    DROP COLUMN IF EXISTS inherit_to_children,
    DROP COLUMN IF EXISTS principal_id,
    DROP COLUMN IF EXISTS principal_type,
    DROP COLUMN IF EXISTS organization_id;

CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_grants_identity
    ON filesystem.resource_grants(node_id, grantee_id, permission, effect);

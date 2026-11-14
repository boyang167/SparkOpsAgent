ALTER TABLE filesystem.resource_grants
    ADD COLUMN IF NOT EXISTS organization_id UUID,
    ADD COLUMN IF NOT EXISTS principal_type VARCHAR(32),
    ADD COLUMN IF NOT EXISTS principal_id UUID,
    ADD COLUMN IF NOT EXISTS inherit_to_children BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS policy_version BIGINT NOT NULL DEFAULT 1;

UPDATE filesystem.resource_grants rg
SET organization_id = nsp.organization_id,
    principal_type = 'USER',
    principal_id = rg.grantee_id,
    inherit_to_children = TRUE,
    policy_version = 1
FROM filesystem.nodes n
JOIN filesystem.namespaces nsp ON nsp.id = n.namespace_id
WHERE n.id = rg.node_id
  AND (rg.organization_id IS NULL OR rg.principal_type IS NULL OR rg.principal_id IS NULL);

ALTER TABLE filesystem.resource_grants
    ALTER COLUMN organization_id SET NOT NULL,
    ALTER COLUMN principal_type SET NOT NULL,
    ALTER COLUMN principal_id SET NOT NULL;

ALTER TABLE filesystem.resource_grants
    DROP CONSTRAINT IF EXISTS resource_grants_node_id_grantee_id_permission_effect_key;
DROP INDEX IF EXISTS filesystem.ux_filesystem_grants_identity;

CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_grants_active_identity
    ON filesystem.resource_grants(node_id, principal_type, principal_id, permission, effect)
    WHERE revoked_at IS NULL;

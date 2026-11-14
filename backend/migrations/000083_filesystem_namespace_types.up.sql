ALTER TABLE filesystem.namespaces
    ADD COLUMN namespace_type VARCHAR(16) NOT NULL DEFAULT 'WORKSPACE',
    ADD COLUMN owner_user_id UUID,
    ADD COLUMN policy_version BIGINT NOT NULL DEFAULT 1;

UPDATE filesystem.namespaces
SET namespace_type = 'WORKSPACE',
    policy_version = 1
WHERE namespace_type IS NULL OR namespace_type = '';

ALTER TABLE filesystem.namespaces
    DROP CONSTRAINT IF EXISTS namespaces_workspace_id_key,
    DROP CONSTRAINT IF EXISTS ck_filesystem_namespace_type,
    DROP CONSTRAINT IF EXISTS ck_filesystem_namespace_scope,
    ALTER COLUMN workspace_id DROP NOT NULL;

ALTER TABLE filesystem.namespaces
    ADD CONSTRAINT ck_filesystem_namespace_type
        CHECK (namespace_type IN ('ORGANIZATION','WORKSPACE','USER')),
    ADD CONSTRAINT ck_filesystem_namespace_scope
        CHECK (
            (namespace_type = 'ORGANIZATION' AND workspace_id IS NULL AND owner_user_id IS NULL)
            OR (namespace_type = 'WORKSPACE' AND workspace_id IS NOT NULL AND owner_user_id IS NULL)
            OR (namespace_type = 'USER' AND workspace_id IS NULL AND owner_user_id IS NOT NULL)
        );

CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_namespaces_workspace_id_id
    ON filesystem.namespaces(workspace_id, id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_namespaces_organization_id
    ON filesystem.namespaces(organization_id)
    WHERE namespace_type = 'ORGANIZATION';

CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_namespaces_workspace_id
    ON filesystem.namespaces(workspace_id)
    WHERE namespace_type = 'WORKSPACE';

CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_namespaces_user_owner
    ON filesystem.namespaces(organization_id, owner_user_id)
    WHERE namespace_type = 'USER';

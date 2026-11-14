ALTER TABLE filesystem.resource_grants
    DROP CONSTRAINT IF EXISTS resource_grants_permission_check,
    DROP CONSTRAINT IF EXISTS ck_filesystem_grant_values;
ALTER TABLE filesystem.resource_grants
    ADD CONSTRAINT ck_filesystem_grant_values
    CHECK (permission IN ('READ','DOWNLOAD','WRITE','DELETE','SHARE','ADMIN') AND effect IN ('ALLOW','DENY'));

ALTER TABLE filesystem.nodes
    ADD COLUMN classification VARCHAR(16);
UPDATE filesystem.nodes SET classification = 'INTERNAL' WHERE classification IS NULL;
ALTER TABLE filesystem.nodes
    ALTER COLUMN classification SET DEFAULT 'INTERNAL',
    ALTER COLUMN classification SET NOT NULL,
    ADD CONSTRAINT ck_filesystem_node_classification
        CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED'));

CREATE TABLE IF NOT EXISTS filesystem.share_requests (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    namespace_id UUID NOT NULL,
    node_id UUID NOT NULL,
    requested_by UUID NOT NULL,
    principal_type VARCHAR(32) NOT NULL,
    principal_ids JSONB NOT NULL,
    actions JSONB NOT NULL,
    inherit_to_children BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    classification VARCHAR(16) NOT NULL,
    effect VARCHAR(8) NOT NULL DEFAULT 'ALLOW',
    status VARCHAR(16) NOT NULL,
    reviewed_by UUID,
    reviewed_at TIMESTAMPTZ,
    rejection_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_share_requests_namespace FOREIGN KEY (namespace_id) REFERENCES filesystem.namespaces(id),
    CONSTRAINT fk_share_requests_node FOREIGN KEY (node_id) REFERENCES filesystem.nodes(id),
    CONSTRAINT ck_share_requests_principal_type CHECK (principal_type IN ('USER','WORKSPACE','ORGANIZATION','ROLE')),
    CONSTRAINT ck_share_requests_actions CHECK (
        actions <@ '["READ","DOWNLOAD","WRITE","DELETE","SHARE","ADMIN"]'::jsonb
        AND jsonb_array_length(actions) > 0
    ),
    CONSTRAINT ck_share_requests_classification CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED')),
    CONSTRAINT ck_share_requests_effect CHECK (effect IN ('ALLOW','DENY')),
    CONSTRAINT ck_share_requests_status CHECK (status IN ('PENDING','APPROVED','REJECTED','CANCELLED'))
);

CREATE INDEX IF NOT EXISTS ix_filesystem_share_requests_org_status
    ON filesystem.share_requests(organization_id, status, created_at);
CREATE INDEX IF NOT EXISTS ix_filesystem_share_requests_node
    ON filesystem.share_requests(node_id, created_at);

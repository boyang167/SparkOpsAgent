ALTER TABLE filesystem.resource_grants
    DROP CONSTRAINT IF EXISTS resource_grants_permission_check,
    DROP CONSTRAINT IF EXISTS ck_filesystem_grant_values;
ALTER TABLE filesystem.resource_grants
    ADD CONSTRAINT ck_filesystem_grant_values
    CHECK (permission IN ('READ','DOWNLOAD','WRITE','DELETE','SHARE','ADMIN') AND effect IN ('ALLOW','DENY'));

ALTER TABLE filesystem.nodes
    ADD COLUMN IF NOT EXISTS classification VARCHAR(16);
UPDATE filesystem.nodes SET classification = 'INTERNAL' WHERE node_type = 'DIRECTORY';
UPDATE filesystem.nodes SET classification = 'RESTRICTED' WHERE node_type = 'FILE';

ALTER TABLE filesystem.transfer_sessions
    ADD COLUMN IF NOT EXISTS committed_node_id UUID REFERENCES filesystem.nodes(id);

CREATE EXTENSION IF NOT EXISTS pgcrypto;
WITH transfer_hashes AS (
    SELECT id,
           encode(digest(
               uuid_send('6ba7b811-9dad-11d1-80b4-00c04fd430c8'::uuid)
               || convert_to('filesystem-transfer-node:' || id::text, 'UTF8'),
               'sha1'
           ), 'hex') AS hash
    FROM filesystem.transfer_sessions
    WHERE status = 'COMPLETED'
), transfer_nodes AS (
    SELECT id,
           (
               substr(hash, 1, 8) || '-' || substr(hash, 9, 4) || '-5' || substr(hash, 14, 3) || '-'
               || CASE lower(substr(hash, 17, 1))
                    WHEN '0' THEN '8' WHEN '1' THEN '8' WHEN '2' THEN '8' WHEN '3' THEN '8'
                    WHEN '4' THEN '9' WHEN '5' THEN '9' WHEN '6' THEN '9' WHEN '7' THEN '9'
                    WHEN '8' THEN 'a' WHEN '9' THEN 'a' WHEN 'a' THEN 'a' WHEN 'b' THEN 'a'
                    ELSE 'b'
                  END
               || substr(hash, 18, 3) || '-' || substr(hash, 21, 12)
           )::uuid AS node_id
    FROM transfer_hashes
)
UPDATE filesystem.transfer_sessions AS session
SET committed_node_id = transfer_nodes.node_id
FROM transfer_nodes
JOIN filesystem.nodes ON filesystem.nodes.id = transfer_nodes.node_id
WHERE session.id = transfer_nodes.id
  AND session.committed_node_id IS NULL;

WITH classified_nodes AS (
    SELECT committed_node_id,
           MAX(CASE classification
                 WHEN 'PUBLIC' THEN 1
                 WHEN 'INTERNAL' THEN 2
                 WHEN 'CONFIDENTIAL' THEN 3
                 WHEN 'RESTRICTED' THEN 4
               END) AS sensitivity
    FROM filesystem.transfer_sessions
    WHERE status = 'COMPLETED' AND committed_node_id IS NOT NULL
    GROUP BY committed_node_id
)
UPDATE filesystem.nodes AS node
SET classification = CASE classified_nodes.sensitivity
    WHEN 1 THEN 'PUBLIC'
    WHEN 2 THEN 'INTERNAL'
    WHEN 3 THEN 'CONFIDENTIAL'
    WHEN 4 THEN 'RESTRICTED'
END
FROM classified_nodes
WHERE node.id = classified_nodes.committed_node_id;

ALTER TABLE filesystem.nodes
    ALTER COLUMN classification SET DEFAULT 'INTERNAL',
    ALTER COLUMN classification SET NOT NULL;
ALTER TABLE filesystem.nodes
    DROP CONSTRAINT IF EXISTS ck_filesystem_node_classification;
ALTER TABLE filesystem.nodes
    ADD CONSTRAINT ck_filesystem_node_classification
        CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED'));

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'filesystem' AND table_name = 'export_tasks'
    ) THEN
        ALTER TABLE filesystem.export_tasks
            ADD COLUMN IF NOT EXISTS classification VARCHAR(16) NOT NULL DEFAULT 'RESTRICTED';
        ALTER TABLE filesystem.export_tasks DROP CONSTRAINT IF EXISTS ck_filesystem_export_classification;
        ALTER TABLE filesystem.export_tasks
            ADD CONSTRAINT ck_filesystem_export_classification
                CHECK (classification IN ('PUBLIC','INTERNAL','CONFIDENTIAL','RESTRICTED'));
    END IF;
END $$;

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

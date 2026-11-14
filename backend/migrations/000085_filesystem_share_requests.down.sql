DROP TABLE IF EXISTS filesystem.share_requests;

-- Irreversible rollback data loss: the previous schema cannot represent DOWNLOAD grants.
DELETE FROM filesystem.resource_grants WHERE permission = 'DOWNLOAD';

ALTER TABLE filesystem.resource_grants
    DROP CONSTRAINT IF EXISTS ck_filesystem_grant_values;
ALTER TABLE filesystem.resource_grants
    ADD CONSTRAINT ck_filesystem_grant_values
    CHECK (permission IN ('READ','WRITE','DELETE','SHARE','ADMIN') AND effect IN ('ALLOW','DENY'));

ALTER TABLE filesystem.nodes
    DROP CONSTRAINT IF EXISTS ck_filesystem_node_classification,
    DROP COLUMN IF EXISTS classification;

ALTER TABLE filesystem.export_tasks
    DROP CONSTRAINT IF EXISTS ck_filesystem_export_classification,
    DROP COLUMN IF EXISTS classification;

ALTER TABLE filesystem.transfer_sessions
    DROP COLUMN IF EXISTS committed_node_id;

-- Irreversible rollback data loss: the previous schema cannot represent DOWNLOAD grants.
-- Production incidents should use a forward-fix. A deliberate rollback must opt in
-- inside the same transaction with:
-- SET LOCAL sparkops.allow_destructive_filesystem_000085_down = 'on';
DO $filesystem_000085_down_guard$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM filesystem.resource_grants
        WHERE permission = 'DOWNLOAD'
    ) AND current_setting('sparkops.allow_destructive_filesystem_000085_down', true) IS DISTINCT FROM 'on' THEN
        RAISE EXCEPTION
            USING ERRCODE = '55000',
                  MESSAGE = 'migration 000085 down would delete DOWNLOAD grants',
                  HINT = 'Prefer a production forward-fix. For a deliberate rollback, use SET LOCAL sparkops.allow_destructive_filesystem_000085_down = ''on'' in the same transaction.';
    END IF;
END
$filesystem_000085_down_guard$;

DELETE FROM filesystem.resource_grants WHERE permission = 'DOWNLOAD';

DROP TABLE IF EXISTS filesystem.share_requests;

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

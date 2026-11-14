DROP TABLE IF EXISTS filesystem.share_requests;

ALTER TABLE filesystem.resource_grants
    DROP CONSTRAINT IF EXISTS ck_filesystem_grant_values;
ALTER TABLE filesystem.resource_grants
    ADD CONSTRAINT ck_filesystem_grant_values
    CHECK (permission IN ('READ','WRITE','DELETE','SHARE','ADMIN') AND effect IN ('ALLOW','DENY'));

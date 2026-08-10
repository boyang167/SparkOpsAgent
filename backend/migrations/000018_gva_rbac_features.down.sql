DROP TABLE IF EXISTS identity.token_blacklist;
DROP TABLE IF EXISTS identity.api_tokens;
DROP TABLE IF EXISTS governance.role_api_policies;
DROP TABLE IF EXISTS platform.api_ignore;
DROP TABLE IF EXISTS platform.apis;
ALTER TABLE governance.roles DROP COLUMN IF EXISTS parent_id;

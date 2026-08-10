DROP INDEX IF EXISTS tenant.idx_workspaces_legacy_namespace;

ALTER TABLE tenant.workspaces
    DROP COLUMN IF EXISTS legacy_namespace;

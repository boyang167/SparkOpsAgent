-- Platform extensions for migration from legacy PlatformSuite namespace model.

ALTER TABLE tenant.workspaces
    ADD COLUMN IF NOT EXISTS legacy_namespace VARCHAR(128);

CREATE INDEX IF NOT EXISTS idx_workspaces_legacy_namespace
    ON tenant.workspaces(legacy_namespace)
    WHERE legacy_namespace IS NOT NULL;

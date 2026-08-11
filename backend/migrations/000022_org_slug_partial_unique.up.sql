-- Ensure slug uniqueness applies only to active (non-deleted) organizations.
ALTER TABLE tenant.organizations DROP CONSTRAINT IF EXISTS organizations_slug_key;

DROP INDEX IF EXISTS tenant.organizations_slug_key;
DROP INDEX IF EXISTS tenant.idx_tenant_organizations_slug;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_organizations_slug_active
    ON tenant.organizations (slug)
    WHERE deleted_at IS NULL;

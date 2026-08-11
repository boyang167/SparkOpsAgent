-- Allow slug reuse after soft delete.
ALTER TABLE tenant.organizations DROP CONSTRAINT IF EXISTS organizations_slug_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_organizations_slug_active
    ON tenant.organizations (slug)
    WHERE deleted_at IS NULL;

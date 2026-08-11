DROP INDEX IF EXISTS tenant.idx_tenant_organizations_slug_active;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_organizations_slug ON tenant.organizations (slug);

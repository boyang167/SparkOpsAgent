DROP INDEX IF EXISTS tenant.idx_tenant_organizations_slug_active;

CREATE UNIQUE INDEX IF NOT EXISTS organizations_slug_key ON tenant.organizations (slug);

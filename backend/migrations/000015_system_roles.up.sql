CREATE TABLE IF NOT EXISTS identity.system_role_bindings (
    member_id UUID NOT NULL PRIMARY KEY,
    role      TEXT NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_system_role_bindings_role ON identity.system_role_bindings (role);

CREATE TABLE IF NOT EXISTS tenant.organization_members (
    org_id    UUID NOT NULL,
    member_id UUID NOT NULL,
    role      TEXT NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (org_id, member_id)
);

CREATE INDEX IF NOT EXISTS idx_org_members_member ON tenant.organization_members (member_id);

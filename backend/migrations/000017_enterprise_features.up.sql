-- Org quotas
ALTER TABLE tenant.organizations
    ADD COLUMN IF NOT EXISTS max_workspaces INT NOT NULL DEFAULT 50,
    ADD COLUMN IF NOT EXISTS max_agents INT NOT NULL DEFAULT 500,
    ADD COLUMN IF NOT EXISTS max_projects INT NOT NULL DEFAULT 1000,
    ADD COLUMN IF NOT EXISTS token_budget BIGINT NOT NULL DEFAULT 0;

ALTER TABLE tenant.workspaces
    ADD COLUMN IF NOT EXISTS token_budget BIGINT NOT NULL DEFAULT 0;

-- External identities (SSO/LDAP)
CREATE TABLE IF NOT EXISTS identity.external_identities (
    provider    TEXT NOT NULL,
    subject     TEXT NOT NULL,
    member_id   UUID NOT NULL,
    email       TEXT NOT NULL,
    linked_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider, subject)
);

CREATE INDEX IF NOT EXISTS idx_external_identities_member ON identity.external_identities (member_id);

-- Project members
CREATE TABLE IF NOT EXISTS workspace_app.project_members (
    project_id  UUID NOT NULL,
    member_id   UUID NOT NULL,
    role        TEXT NOT NULL DEFAULT 'member',
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    PRIMARY KEY (project_id, member_id)
);

CREATE INDEX IF NOT EXISTS idx_project_members_member ON workspace_app.project_members (member_id);

-- Casbin policy rules (governance schema)
CREATE TABLE IF NOT EXISTS governance.casbin_rule (
    id    BIGSERIAL PRIMARY KEY,
    ptype VARCHAR(100) NOT NULL,
    v0    VARCHAR(256),
    v1    VARCHAR(256),
    v2    VARCHAR(256),
    v3    VARCHAR(256),
    v4    VARCHAR(256),
    v5    VARCHAR(256)
);

CREATE INDEX IF NOT EXISTS idx_casbin_rule_ptype ON governance.casbin_rule (ptype);

-- Backfill project members from existing projects
INSERT INTO workspace_app.project_members (project_id, member_id, role, joined_at, created_at, updated_at)
SELECT p.id, p.created_by, 'lead', p.created_at, p.created_at, p.created_at
FROM workspace_app.projects p
WHERE p.status = 'active' AND p.deleted_at IS NULL
ON CONFLICT (project_id, member_id) DO NOTHING;

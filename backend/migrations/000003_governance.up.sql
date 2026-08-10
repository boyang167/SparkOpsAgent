CREATE SCHEMA IF NOT EXISTS governance;

CREATE TABLE IF NOT EXISTS governance.roles (
    id           UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    name         VARCHAR(64) NOT NULL,
    permissions  JSONB NOT NULL DEFAULT '[]',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, name)
);

CREATE TABLE IF NOT EXISTS governance.role_bindings (
    id         UUID PRIMARY KEY,
    role_id    UUID NOT NULL,
    member_id  UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(role_id, member_id)
);

CREATE INDEX IF NOT EXISTS idx_role_bindings_member ON governance.role_bindings(member_id);

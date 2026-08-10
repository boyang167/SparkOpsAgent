-- Role tree
ALTER TABLE governance.roles ADD COLUMN IF NOT EXISTS parent_id UUID NULL;
CREATE INDEX IF NOT EXISTS idx_governance_roles_parent ON governance.roles(parent_id);

-- Platform API registry (GVA-style sync)
CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.apis (
    id           UUID PRIMARY KEY,
    path         VARCHAR(512) NOT NULL,
    method       VARCHAR(16) NOT NULL,
    api_group    VARCHAR(64) NOT NULL DEFAULT '',
    description  VARCHAR(512) NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(path, method)
);

CREATE TABLE IF NOT EXISTS platform.api_ignore (
    path   VARCHAR(512) NOT NULL,
    method VARCHAR(16) NOT NULL,
    PRIMARY KEY(path, method)
);

-- Workspace role -> API policy (Casbin-style, workspace scope)
CREATE TABLE IF NOT EXISTS governance.role_api_policies (
    role_id UUID NOT NULL,
    path    VARCHAR(512) NOT NULL,
    method  VARCHAR(16) NOT NULL,
    PRIMARY KEY(role_id, path, method)
);

-- API tokens (GVA-style long-lived JWT)
CREATE TABLE IF NOT EXISTS identity.api_tokens (
    id           UUID PRIMARY KEY,
    member_id    UUID NOT NULL,
    workspace_id UUID NULL,
    role_id      UUID NULL,
    token_prefix VARCHAR(16) NOT NULL,
    status       VARCHAR(16) NOT NULL DEFAULT 'active',
    expires_at   TIMESTAMPTZ NULL,
    remark       VARCHAR(255) NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_member ON identity.api_tokens(member_id);

CREATE TABLE IF NOT EXISTS identity.token_blacklist (
    token_hash VARCHAR(64) PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

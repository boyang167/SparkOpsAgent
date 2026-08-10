CREATE SCHEMA IF NOT EXISTS tenant;

CREATE TABLE IF NOT EXISTS tenant.organizations (
    id         UUID PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    slug       VARCHAR(64) NOT NULL UNIQUE,
    status     VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tenant.workspaces (
    id           UUID PRIMARY KEY,
    org_id       UUID NOT NULL,
    name         VARCHAR(255) NOT NULL,
    slug         VARCHAR(64) NOT NULL,
    status       VARCHAR(32) NOT NULL DEFAULT 'active',
    max_agents   INT NOT NULL DEFAULT 10,
    max_projects INT NOT NULL DEFAULT 20,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, slug)
);

CREATE TABLE IF NOT EXISTS tenant.workspace_members (
    workspace_id UUID NOT NULL,
    member_id    UUID NOT NULL,
    role         VARCHAR(64) NOT NULL DEFAULT 'member',
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, member_id)
);

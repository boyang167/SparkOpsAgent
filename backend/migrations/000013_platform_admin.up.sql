CREATE SCHEMA IF NOT EXISTS platform_admin;

CREATE TABLE IF NOT EXISTS platform_admin.sys_cfg (
    id            UUID PRIMARY KEY,
    kind          VARCHAR(64) NOT NULL,
    name          VARCHAR(255) NOT NULL,
    workspace_id  UUID,
    whether_share BOOLEAN NOT NULL DEFAULT FALSE,
    config        JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
    UNIQUE (kind, name, workspace_id)
);

CREATE TABLE IF NOT EXISTS platform_admin.cluster_info (
    id                         UUID PRIMARY KEY,
    cluster_name               VARCHAR(255) NOT NULL UNIQUE,
    server                     VARCHAR(512),
    user_name                  VARCHAR(255),
    certificate_authority_data TEXT,
    client_certificate_data    TEXT,
    client_key_data            TEXT,
    is_build_env               BOOLEAN NOT NULL DEFAULT FALSE,
    priority_id                INT NOT NULL DEFAULT 0,
    created_by                 UUID NOT NULL,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at                 TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS platform_admin.sync_jobs (
    id             UUID PRIMARY KEY,
    name           VARCHAR(255) NOT NULL,
    workspace_id   UUID,
    version        VARCHAR(64),
    source_cluster VARCHAR(255),
    target_cluster VARCHAR(255),
    sync_resources TEXT,
    status         VARCHAR(32) NOT NULL DEFAULT 'pending',
    result         TEXT,
    created_by     UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sync_jobs_workspace ON platform_admin.sync_jobs(workspace_id);

CREATE TABLE IF NOT EXISTS platform_admin.statistics_cache (
    id           UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    cache_key    VARCHAR(255) NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}',
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, cache_key)
);

CREATE TABLE IF NOT EXISTS platform_admin.proclaims (
    id               UUID PRIMARY KEY,
    title            VARCHAR(512) NOT NULL,
    body             TEXT,
    proclaim_type    VARCHAR(64),
    proclaim_receive VARCHAR(255),
    status           VARCHAR(32) NOT NULL DEFAULT 'active',
    created_by       UUID NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ
);

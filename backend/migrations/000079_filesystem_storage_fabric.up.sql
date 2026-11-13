CREATE TABLE filesystem.storage_providers (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    provider_type VARCHAR(16) NOT NULL CHECK (provider_type IN ('LOCAL','NFS','S3')),
    endpoint TEXT NOT NULL DEFAULT '',
    secret_ref TEXT NOT NULL CHECK (length(btrim(secret_ref)) > 0),
    capabilities JSONB NOT NULL DEFAULT '{}'::jsonb,
    health JSONB NOT NULL DEFAULT '{}'::jsonb,
    config_version BIGINT NOT NULL DEFAULT 1 CHECK (config_version > 0),
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, name)
);

CREATE TABLE filesystem.storage_pools (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
    placement_policy_version BIGINT NOT NULL DEFAULT 0 CHECK (placement_policy_version >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, name)
);

CREATE TABLE filesystem.storage_pool_members (
    id UUID PRIMARY KEY,
    pool_id UUID NOT NULL,
    provider_id UUID NOT NULL,
    priority INT NOT NULL DEFAULT 0,
    mode VARCHAR(16) NOT NULL CHECK (mode IN ('PRIMARY','REPLICA','ARCHIVE','DRAINING')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (pool_id, provider_id),
    CONSTRAINT fk_pool_members_pool
        FOREIGN KEY (pool_id) REFERENCES filesystem.storage_pools(id),
    CONSTRAINT fk_pool_members_provider
        FOREIGN KEY (provider_id) REFERENCES filesystem.storage_providers(id)
);

CREATE INDEX ix_storage_pool_members_pool_priority
    ON filesystem.storage_pool_members(pool_id, priority, mode);

CREATE TABLE filesystem.placement_policies (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    rules JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, name, version)
);

CREATE TABLE filesystem.storage_bindings (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    scope_type VARCHAR(16) NOT NULL CHECK (scope_type IN ('ORGANIZATION','WORKSPACE','PROJECT','USER')),
    scope_id UUID NOT NULL,
    pool_id UUID NOT NULL,
    priority INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, scope_type, scope_id, priority),
    CONSTRAINT fk_storage_bindings_pool
        FOREIGN KEY (pool_id) REFERENCES filesystem.storage_pools(id)
);

CREATE INDEX ix_storage_bindings_scope
    ON filesystem.storage_bindings(scope_type, scope_id);

CREATE TABLE filesystem.mounts (
    id UUID PRIMARY KEY,
    namespace_id UUID NOT NULL,
    mount_path TEXT NOT NULL CHECK (length(btrim(mount_path)) > 0 AND left(mount_path, 1) = '/'),
    pool_id UUID NOT NULL,
    access_mode VARCHAR(16) NOT NULL CHECK (access_mode IN ('READ_ONLY','READ_WRITE')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (namespace_id, mount_path),
    CONSTRAINT fk_mounts_namespace
        FOREIGN KEY (namespace_id) REFERENCES filesystem.namespaces(id),
    CONSTRAINT fk_mounts_pool
        FOREIGN KEY (pool_id) REFERENCES filesystem.storage_pools(id)
);

CREATE TABLE filesystem.lifecycle_policies (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    rules JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, name, version)
);

CREATE TABLE filesystem.lifecycle_jobs (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    policy_id UUID,
    job_type VARCHAR(16) NOT NULL CHECK (job_type IN ('MIGRATE','REPLICATE','ARCHIVE','GC')),
    status VARCHAR(16) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','RUNNING','COMPLETED','FAILED','CANCELLED')),
    idempotency_key TEXT NOT NULL CHECK (length(btrim(idempotency_key)) > 0),
    lease_owner TEXT,
    lease_expires_at TIMESTAMPTZ,
    checkpoint JSONB NOT NULL DEFAULT '{}'::jsonb,
    attempt_count INT NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    max_attempts INT NOT NULL DEFAULT 10 CHECK (max_attempts > 0),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, idempotency_key),
    CONSTRAINT fk_lifecycle_jobs_policy
        FOREIGN KEY (policy_id) REFERENCES filesystem.lifecycle_policies(id)
);

CREATE INDEX ix_lifecycle_jobs_status
    ON filesystem.lifecycle_jobs(organization_id, status);

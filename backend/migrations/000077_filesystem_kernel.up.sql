CREATE SCHEMA IF NOT EXISTS filesystem;

CREATE TABLE IF NOT EXISTS filesystem.namespaces (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    workspace_id UUID NOT NULL UNIQUE,
    root_node_id UUID,
    status VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS filesystem.nodes (
    id UUID PRIMARY KEY,
    namespace_id UUID NOT NULL REFERENCES filesystem.namespaces(id),
    parent_id UUID,
    node_type VARCHAR(16) NOT NULL CHECK (node_type IN ('FILE','DIRECTORY')),
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    current_revision_id UUID,
    state VARCHAR(32) NOT NULL,
    owner_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(id, namespace_id),
    UNIQUE(namespace_id, parent_id, normalized_name)
);

ALTER TABLE filesystem.nodes
    ADD CONSTRAINT fk_nodes_parent_namespace
    FOREIGN KEY (parent_id, namespace_id)
    REFERENCES filesystem.nodes(id, namespace_id);

CREATE TABLE IF NOT EXISTS filesystem.blobs (
    id UUID PRIMARY KEY,
    dedup_domain UUID NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    size BIGINT NOT NULL CHECK (size >= 0),
    encryption_key_ref TEXT,
    ref_count BIGINT NOT NULL DEFAULT 0 CHECK (ref_count >= 0),
    state VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(dedup_domain, sha256, size)
);

CREATE TABLE IF NOT EXISTS filesystem.file_revisions (
    id UUID PRIMARY KEY,
    node_id UUID NOT NULL REFERENCES filesystem.nodes(id),
    revision_no INT NOT NULL CHECK (revision_no > 0),
    blob_id UUID NOT NULL REFERENCES filesystem.blobs(id),
    size BIGINT NOT NULL CHECK (size >= 0),
    mime_type VARCHAR(255) NOT NULL,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(id, node_id),
    UNIQUE(node_id, revision_no)
);

CREATE TABLE IF NOT EXISTS filesystem.storage_replicas (
    id UUID PRIMARY KEY,
    blob_id UUID NOT NULL REFERENCES filesystem.blobs(id),
    provider_id UUID NOT NULL,
    object_key TEXT NOT NULL,
    checksum VARCHAR(128),
    state VARCHAR(32) NOT NULL,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(blob_id, provider_id, object_key)
);

CREATE TABLE IF NOT EXISTS filesystem.quota_accounts (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    workspace_id UUID NOT NULL UNIQUE,
    used_bytes BIGINT NOT NULL DEFAULT 0 CHECK (used_bytes >= 0),
    reserved_bytes BIGINT NOT NULL DEFAULT 0 CHECK (reserved_bytes >= 0),
    used_files BIGINT NOT NULL DEFAULT 0 CHECK (used_files >= 0),
    reserved_files BIGINT NOT NULL DEFAULT 0 CHECK (reserved_files >= 0),
    version BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS filesystem.quota_reservations (
    id UUID PRIMARY KEY,
    quota_account_id UUID NOT NULL REFERENCES filesystem.quota_accounts(id),
    bytes BIGINT NOT NULL CHECK (bytes >= 0),
    files BIGINT NOT NULL CHECK (files >= 0),
    state VARCHAR(32) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS filesystem.resource_grants (
    id UUID PRIMARY KEY,
    node_id UUID NOT NULL REFERENCES filesystem.nodes(id),
    grantee_id UUID NOT NULL,
    permission VARCHAR(32) NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(node_id, grantee_id, permission)
);

CREATE TABLE IF NOT EXISTS filesystem.file_links (
    id UUID PRIMARY KEY,
    node_id UUID NOT NULL REFERENCES filesystem.nodes(id),
    revision_id UUID REFERENCES filesystem.file_revisions(id),
    link_type VARCHAR(64) NOT NULL,
    link_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(link_type, link_id, node_id, revision_id)
);

ALTER TABLE filesystem.nodes
    ADD CONSTRAINT fk_nodes_current_revision
    FOREIGN KEY (current_revision_id, id)
    REFERENCES filesystem.file_revisions(id, node_id);

ALTER TABLE filesystem.namespaces
    ADD CONSTRAINT fk_namespaces_root_node
    FOREIGN KEY (root_node_id, id)
    REFERENCES filesystem.nodes(id, namespace_id);

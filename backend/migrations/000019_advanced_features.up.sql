-- Consul KV mock store (greenfield; replace with real Consul adapter when configured)
CREATE TABLE IF NOT EXISTS platform.consul_kv (
    workspace_id UUID NOT NULL,
    key          VARCHAR(512) NOT NULL,
    value        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, key)
);

CREATE INDEX IF NOT EXISTS idx_consul_kv_prefix ON platform.consul_kv(workspace_id, key varchar_pattern_ops);

CREATE TABLE IF NOT EXISTS platform.consul_kv (
    id          bigserial PRIMARY KEY,
    workspace_id varchar(36) NOT NULL,
    key         text NOT NULL,
    value       text NOT NULL DEFAULT '',
    updated_at  timestamptz NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, key)
);

CREATE INDEX IF NOT EXISTS idx_consul_kv_prefix ON platform.consul_kv(workspace_id, key varchar_pattern_ops);

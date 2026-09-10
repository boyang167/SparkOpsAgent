CREATE TABLE IF NOT EXISTS delivery.project_webhooks (
    id           UUID PRIMARY KEY,
    project_id   UUID NOT NULL,
    name         VARCHAR(255) NOT NULL,
    url          VARCHAR(1024) NOT NULL,
    token        VARCHAR(512) NOT NULL DEFAULT '',
    events       JSONB NOT NULL DEFAULT '[]',
    enabled      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_project_webhooks_project ON delivery.project_webhooks(project_id);
CREATE INDEX IF NOT EXISTS idx_project_webhooks_enabled ON delivery.project_webhooks(project_id, enabled);

CREATE TABLE IF NOT EXISTS workspace_app.secrets (
    id              UUID PRIMARY KEY,
    workspace_id    UUID NOT NULL,
    name            VARCHAR(255) NOT NULL,
    display_name    VARCHAR(255),
    description     TEXT,
    service_type    VARCHAR(64) NOT NULL,
    service_url     VARCHAR(512) NOT NULL,
    username        VARCHAR(255),
    credential_blob TEXT NOT NULL DEFAULT '',
    status          VARCHAR(32) NOT NULL DEFAULT 'active',
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    is_shared       BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_secrets_workspace ON workspace_app.secrets(workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_secrets_workspace_name ON workspace_app.secrets(workspace_id, name) WHERE deleted_at IS NULL;

-- workspace_app extensions: templates and dockerfile market

CREATE TABLE IF NOT EXISTS workspace_app.templates (
    id           UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    name         VARCHAR(255) NOT NULL,
    kind         VARCHAR(255),
    language     VARCHAR(64),
    detail       TEXT,
    rules        TEXT,
    created_by   UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_templates_workspace ON workspace_app.templates(workspace_id);

CREATE TABLE IF NOT EXISTS workspace_app.dockerfiles (
    id             UUID PRIMARY KEY,
    workspace_id   UUID NOT NULL,
    name           VARCHAR(255) NOT NULL,
    language       VARCHAR(64),
    description    TEXT,
    detail         TEXT,
    dockerfile     TEXT,
    version        VARCHAR(64),
    full_path      VARCHAR(512),
    sha256         VARCHAR(128),
    download_count INT NOT NULL DEFAULT 0,
    score          INT NOT NULL DEFAULT 0,
    status         VARCHAR(32) NOT NULL DEFAULT 'active',
    created_by     UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_dockerfiles_workspace ON workspace_app.dockerfiles(workspace_id);

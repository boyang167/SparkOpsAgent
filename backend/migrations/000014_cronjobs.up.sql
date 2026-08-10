-- Optional cronjob stub table for future scheduled build triggers.

CREATE TABLE IF NOT EXISTS delivery.cronjobs (
    id           UUID PRIMARY KEY,
    project_id   UUID NOT NULL,
    name         VARCHAR(255) NOT NULL,
    schedule     VARCHAR(128) NOT NULL,
    branch       VARCHAR(255) NOT NULL DEFAULT 'main',
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_cronjobs_project ON delivery.cronjobs(project_id);

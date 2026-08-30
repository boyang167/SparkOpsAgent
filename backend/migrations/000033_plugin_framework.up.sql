CREATE SCHEMA IF NOT EXISTS plugin;

CREATE TABLE plugin.plugins (
    id            TEXT PRIMARY KEY,
    version       TEXT NOT NULL,
    display_name  TEXT NOT NULL,
    manifest_yaml TEXT NOT NULL,
    package_digest TEXT NOT NULL DEFAULT '',
    installed_files JSONB NOT NULL DEFAULT '[]'::jsonb,
    status        TEXT NOT NULL,
    pr_url        TEXT NOT NULL DEFAULT '',
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE plugin.workspace_bindings (
    workspace_id UUID NOT NULL,
    plugin_id    TEXT NOT NULL REFERENCES plugin.plugins(id),
    enabled      BOOLEAN NOT NULL DEFAULT FALSE,
    enabled_by   UUID,
    enabled_at   TIMESTAMPTZ,
    PRIMARY KEY (workspace_id, plugin_id)
);

CREATE TABLE plugin.uninstall_receipts (
    plugin_id   TEXT PRIMARY KEY,
    deployed_at TIMESTAMPTZ NOT NULL
);

-- Formal DDL for BCs previously created only via GORM AutoMigrate at runtime.

CREATE SCHEMA IF NOT EXISTS workflow;
CREATE SCHEMA IF NOT EXISTS knowledge;
CREATE SCHEMA IF NOT EXISTS notification;
CREATE SCHEMA IF NOT EXISTS delivery;

CREATE TABLE IF NOT EXISTS workflow.workflows (
    id           UUID PRIMARY KEY,
    project_id   UUID NOT NULL,
    name         VARCHAR(255) NOT NULL,
    description  TEXT,
    status       VARCHAR(32) NOT NULL,
    definition   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_workflows_project ON workflow.workflows(project_id);

CREATE TABLE IF NOT EXISTS workflow.workflow_runs (
    id           UUID PRIMARY KEY,
    workflow_id  UUID NOT NULL,
    member_id    UUID NOT NULL,
    workspace_id UUID NOT NULL,
    status       VARCHAR(32) NOT NULL,
    input        TEXT,
    output       TEXT,
    error        TEXT,
    started_at   TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_workflow_runs_workflow ON workflow.workflow_runs(workflow_id);

CREATE TABLE IF NOT EXISTS knowledge.knowledge_bases (
    id           UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    name         VARCHAR(255) NOT NULL,
    description  TEXT,
    status       VARCHAR(32) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_knowledge_bases_workspace ON knowledge.knowledge_bases(workspace_id);

CREATE TABLE IF NOT EXISTS knowledge.documents (
    id                UUID PRIMARY KEY,
    knowledge_base_id UUID NOT NULL,
    title             VARCHAR(512) NOT NULL,
    content           TEXT,
    status            VARCHAR(32) NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_documents_kb ON knowledge.documents(knowledge_base_id);

CREATE TABLE IF NOT EXISTS notification.notifications (
    id           UUID PRIMARY KEY,
    member_id    UUID NOT NULL,
    workspace_id UUID NOT NULL,
    type         VARCHAR(64) NOT NULL,
    title        VARCHAR(512) NOT NULL,
    body         TEXT,
    link         TEXT,
    read_at      TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_notifications_member_ws ON notification.notifications(member_id, workspace_id);

CREATE TABLE IF NOT EXISTS delivery.builds (
    id                   UUID PRIMARY KEY,
    project_id           UUID NOT NULL,
    member_id            UUID NOT NULL,
    workspace_id         UUID NOT NULL,
    status               VARCHAR(32) NOT NULL,
    branch               VARCHAR(255) NOT NULL,
    build_number         INT NOT NULL,
    jenkins_job          VARCHAR(512),
    console_log          TEXT,
    error                TEXT,
    rollback_of_build_id VARCHAR(64),
    started_at           TIMESTAMPTZ NOT NULL,
    completed_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_builds_project ON delivery.builds(project_id);

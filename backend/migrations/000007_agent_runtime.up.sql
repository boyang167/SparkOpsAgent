CREATE SCHEMA IF NOT EXISTS agent_runtime;

CREATE TABLE IF NOT EXISTS agent_runtime.conversations (
    id           UUID PRIMARY KEY,
    agent_id     UUID NOT NULL,
    member_id    UUID NOT NULL,
    workspace_id UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_runtime.messages (
    id              UUID PRIMARY KEY,
    conversation_id UUID NOT NULL,
    role            VARCHAR(16) NOT NULL,
    content         TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_runtime.runs (
    id                UUID PRIMARY KEY,
    conversation_id   UUID NOT NULL,
    agent_version_id  UUID NOT NULL,
    status            VARCHAR(32) NOT NULL,
    input             TEXT NOT NULL,
    output            TEXT,
    prompt_tokens     INT DEFAULT 0,
    completion_tokens INT DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_messages_conversation ON agent_runtime.messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_runs_conversation ON agent_runtime.runs(conversation_id);

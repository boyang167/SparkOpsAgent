CREATE SCHEMA IF NOT EXISTS agent;

CREATE TABLE IF NOT EXISTS agent.agents (
    id                 UUID PRIMARY KEY,
    project_id         UUID NOT NULL,
    name               VARCHAR(255) NOT NULL,
    status             VARCHAR(32) NOT NULL DEFAULT 'draft',
    current_version_id UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, name)
);

CREATE TABLE IF NOT EXISTS agent.agent_versions (
    id            UUID PRIMARY KEY,
    agent_id      UUID NOT NULL,
    version       INT NOT NULL,
    system_prompt TEXT NOT NULL,
    model_name    VARCHAR(128) NOT NULL,
    model_config  JSONB NOT NULL DEFAULT '{}',
    tool_ids      JSONB NOT NULL DEFAULT '[]',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(agent_id, version)
);

CREATE INDEX IF NOT EXISTS idx_agents_project ON agent.agents(project_id);
CREATE INDEX IF NOT EXISTS idx_agent_versions_agent ON agent.agent_versions(agent_id);

ALTER TABLE agent_runtime.runs DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;
ALTER TABLE agent_runtime.messages DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;
ALTER TABLE agent_runtime.conversations DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;

ALTER TABLE agent.agent_versions DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;
ALTER TABLE agent.agents DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;

ALTER TABLE workspace_app.projects DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;

ALTER TABLE governance.role_bindings DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;
ALTER TABLE governance.roles DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;

ALTER TABLE tenant.workspace_members DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at, DROP COLUMN IF EXISTS created_at;
ALTER TABLE tenant.workspaces DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;
ALTER TABLE tenant.organizations DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS updated_at;

ALTER TABLE identity.members DROP COLUMN IF EXISTS deleted_at;

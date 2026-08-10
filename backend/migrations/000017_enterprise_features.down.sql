DROP TABLE IF EXISTS governance.casbin_rule;
DROP TABLE IF EXISTS workspace_app.project_members;
DROP TABLE IF EXISTS identity.external_identities;
ALTER TABLE tenant.workspaces DROP COLUMN IF EXISTS token_budget;
ALTER TABLE tenant.organizations
    DROP COLUMN IF EXISTS max_workspaces,
    DROP COLUMN IF EXISTS max_agents,
    DROP COLUMN IF EXISTS max_projects,
    DROP COLUMN IF EXISTS token_budget;

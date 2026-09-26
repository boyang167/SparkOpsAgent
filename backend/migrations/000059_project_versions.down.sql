ALTER TABLE delivery.deployment_runs DROP COLUMN IF EXISTS project_version_id;
DROP TABLE IF EXISTS workspace_app.project_versions;

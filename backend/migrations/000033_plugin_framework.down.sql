DROP TABLE IF EXISTS plugin.uninstall_receipts;
DROP TABLE IF EXISTS plugin.workspace_bindings;
ALTER TABLE IF EXISTS plugin.plugins DROP COLUMN IF EXISTS package_digest;
ALTER TABLE IF EXISTS plugin.plugins DROP COLUMN IF EXISTS installed_files;
DROP TABLE IF EXISTS plugin.plugins;
DROP SCHEMA IF EXISTS plugin;

CREATE SCHEMA IF NOT EXISTS sprint_process;

ALTER TABLE IF EXISTS product.sprint_processes SET SCHEMA sprint_process;

ALTER TABLE IF EXISTS product.workspace_sprint_process_templates SET SCHEMA sprint_process;

DROP TABLE IF EXISTS product.workspace_sprint_process_templates;

ALTER TABLE product.sprint_processes
    DROP COLUMN IF EXISTS template_key,
    DROP COLUMN IF EXISTS description;

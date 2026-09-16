DROP INDEX IF EXISTS workspace_app.idx_projects_product_id;

ALTER TABLE workspace_app.projects
    DROP COLUMN IF EXISTS product_id;

ALTER TABLE product.products
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS global_variables,
    DROP COLUMN IF EXISTS product_type;

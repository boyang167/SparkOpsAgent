ALTER TABLE product.products DROP COLUMN IF EXISTS default_workflow_id;
DROP TABLE IF EXISTS product.workflow_build_refs;
DROP TABLE IF EXISTS product.product_workflow_runs;
DROP TABLE IF EXISTS product.product_workflows;

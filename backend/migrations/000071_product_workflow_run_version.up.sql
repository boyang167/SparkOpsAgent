ALTER TABLE product.product_workflow_runs
    ADD COLUMN IF NOT EXISTS version BIGINT;

UPDATE product.product_workflow_runs
SET version = 0
WHERE version IS NULL;

ALTER TABLE product.product_workflow_runs
    ALTER COLUMN version SET DEFAULT 0,
    ALTER COLUMN version SET NOT NULL;

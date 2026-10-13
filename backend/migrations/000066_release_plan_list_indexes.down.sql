CREATE INDEX IF NOT EXISTS idx_release_plans_product ON product.release_plans(product_id);

DROP INDEX IF EXISTS product.idx_release_plans_sprint;

DROP INDEX IF EXISTS product.idx_release_plans_product_created;

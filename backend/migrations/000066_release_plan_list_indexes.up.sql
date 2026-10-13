CREATE INDEX IF NOT EXISTS idx_release_plans_product_created
    ON product.release_plans(product_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_release_plans_sprint
    ON product.release_plans(sprint_id)
    WHERE sprint_id IS NOT NULL AND deleted_at IS NULL;

DROP INDEX IF EXISTS product.idx_release_plans_product;

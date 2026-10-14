-- Soft-deleted sprints must not block reusing their name.
ALTER TABLE product.sprints DROP CONSTRAINT IF EXISTS sprints_product_id_name_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_sprints_product_name_active
    ON product.sprints(product_id, name)
    WHERE deleted_at IS NULL;

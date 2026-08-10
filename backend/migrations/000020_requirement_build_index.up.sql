CREATE INDEX IF NOT EXISTS idx_product_requirements_build_id
    ON product.product_requirements (build_id)
    WHERE build_id IS NOT NULL AND build_id <> '';

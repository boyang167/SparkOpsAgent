-- Product v2 Phase 1: extend products + optional project membership

ALTER TABLE product.products
    ADD COLUMN IF NOT EXISTS product_type VARCHAR(32) DEFAULT 'k8s_yaml',
    ADD COLUMN IF NOT EXISTS global_variables JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS status VARCHAR(16) NOT NULL DEFAULT 'active';

ALTER TABLE workspace_app.projects
    ADD COLUMN IF NOT EXISTS product_id UUID NULL REFERENCES product.products(id);

CREATE INDEX IF NOT EXISTS idx_projects_product_id ON workspace_app.projects(product_id);

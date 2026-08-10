CREATE SCHEMA IF NOT EXISTS product;

CREATE TABLE IF NOT EXISTS product.products (
    id           UUID PRIMARY KEY,
    workspace_id UUID NOT NULL,
    name         VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    description  TEXT,
    version      VARCHAR(64),
    next_version VARCHAR(64),
    created_by   UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_products_workspace ON product.products(workspace_id);

CREATE TABLE IF NOT EXISTS product.product_plans (
    id          UUID PRIMARY KEY,
    product_id  UUID NOT NULL REFERENCES product.products(id),
    name        VARCHAR(255) NOT NULL,
    version     VARCHAR(64),
    status      VARCHAR(32) NOT NULL DEFAULT 'draft',
    description TEXT,
    submitted   BOOLEAN NOT NULL DEFAULT FALSE,
    other_info  JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_product_plans_product ON product.product_plans(product_id);

CREATE TABLE IF NOT EXISTS product.product_requirements (
    id                  UUID PRIMARY KEY,
    product_id          UUID NOT NULL REFERENCES product.products(id),
    product_plan_id     UUID REFERENCES product.product_plans(id),
    project_id          UUID,
    project_name        VARCHAR(255),
    project_tag         VARCHAR(255),
    build_id            UUID,
    description         TEXT,
    status              VARCHAR(32) NOT NULL DEFAULT 'pending',
    multicluster_status VARCHAR(32),
    operator            VARCHAR(255),
    other_info          JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_product_requirements_product ON product.product_requirements(product_id);
CREATE INDEX IF NOT EXISTS idx_product_requirements_plan ON product.product_requirements(product_plan_id);

-- Product v2 Phase P2: environments

CREATE TABLE IF NOT EXISTS product.environments (
    id              UUID PRIMARY KEY,
    product_id      UUID NOT NULL REFERENCES product.products(id),
    workspace_id    UUID NOT NULL,
    name            VARCHAR(64) NOT NULL,
    env_type        VARCHAR(16) NOT NULL DEFAULT 'test',
    cluster_id      UUID NOT NULL,
    namespace       VARCHAR(255) NOT NULL,
    variables       JSONB NOT NULL DEFAULT '{}',
    status          VARCHAR(16) NOT NULL DEFAULT 'active',
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    UNIQUE (product_id, name)
);

CREATE TABLE IF NOT EXISTS product.environment_projects (
    environment_id              UUID NOT NULL REFERENCES product.environments(id) ON DELETE CASCADE,
    project_id                  UUID NOT NULL,
    enabled                     BOOLEAN NOT NULL DEFAULT TRUE,
    current_image_tag           VARCHAR(255),
    current_deployment_run_id   UUID,
    last_deployed_at            TIMESTAMPTZ,
    PRIMARY KEY (environment_id, project_id)
);

ALTER TABLE delivery.deployment_runs
    ADD COLUMN IF NOT EXISTS environment_id UUID NULL,
    ADD COLUMN IF NOT EXISTS deploy_namespace VARCHAR(255) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_environments_product ON product.environments(product_id);
CREATE INDEX IF NOT EXISTS idx_environment_projects_env ON product.environment_projects(environment_id);
CREATE INDEX IF NOT EXISTS idx_deployment_runs_environment ON delivery.deployment_runs(environment_id);

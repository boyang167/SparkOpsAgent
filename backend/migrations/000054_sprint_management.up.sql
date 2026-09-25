CREATE TABLE IF NOT EXISTS product.sprint_processes (
    id            UUID PRIMARY KEY,
    product_id    UUID NOT NULL REFERENCES product.products(id),
    name          VARCHAR(255) NOT NULL,
    version       INT NOT NULL DEFAULT 1,
    is_default    BOOLEAN NOT NULL DEFAULT TRUE,
    stages        JSONB NOT NULL DEFAULT '[]',
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sprint_process_product_default
    ON product.sprint_processes(product_id) WHERE is_default = TRUE;

CREATE TABLE IF NOT EXISTS product.sprints (
    id              UUID PRIMARY KEY,
    product_id      UUID NOT NULL REFERENCES product.products(id),
    process_id      UUID NOT NULL REFERENCES product.sprint_processes(id),
    process_version INT NOT NULL,
    stages          JSONB NOT NULL DEFAULT '[]',
    name            VARCHAR(255) NOT NULL,
    goal            TEXT,
    status          VARCHAR(32) NOT NULL DEFAULT 'planning',
    planned_start   TIMESTAMPTZ,
    planned_end     TIMESTAMPTZ,
    actual_start    TIMESTAMPTZ,
    actual_end      TIMESTAMPTZ,
    owner_id        UUID,
    external_ref    JSONB,
    legacy_plan_id  UUID REFERENCES product.product_plans(id),
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    UNIQUE (product_id, name)
);

CREATE INDEX IF NOT EXISTS idx_sprints_product_status ON product.sprints(product_id, status);

CREATE TABLE IF NOT EXISTS product.work_items (
    id             UUID PRIMARY KEY,
    sprint_id      UUID NOT NULL REFERENCES product.sprints(id) ON DELETE CASCADE,
    stage_id       UUID NOT NULL,
    parent_id      UUID REFERENCES product.work_items(id),
    sort_order     INT NOT NULL DEFAULT 0,
    type           VARCHAR(32) NOT NULL DEFAULT 'story',
    title          VARCHAR(512) NOT NULL,
    description    TEXT,
    priority       VARCHAR(16) NOT NULL DEFAULT 'medium',
    status         VARCHAR(32) NOT NULL DEFAULT 'open',
    assignee_id    UUID,
    reporter_id    UUID,
    project_ids    JSONB NOT NULL DEFAULT '[]',
    labels         JSONB NOT NULL DEFAULT '[]',
    external_ref   JSONB,
    delivery       JSONB NOT NULL DEFAULT '{}',
    blocked_reason TEXT,
    version        INT NOT NULL DEFAULT 0,
    created_by     UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_work_items_board ON product.work_items(sprint_id, stage_id, sort_order);

ALTER TABLE product.release_plans
    ADD COLUMN IF NOT EXISTS sprint_id UUID REFERENCES product.sprints(id);

CREATE INDEX IF NOT EXISTS idx_release_plans_sprint ON product.release_plans(sprint_id);

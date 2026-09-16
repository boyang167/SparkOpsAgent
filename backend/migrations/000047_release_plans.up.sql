-- Product v2 Phase R1: release plans

CREATE TABLE IF NOT EXISTS product.release_plans (
    id                    UUID PRIMARY KEY,
    product_id            UUID NOT NULL REFERENCES product.products(id),
    product_plan_id       UUID REFERENCES product.product_plans(id),
    name                  VARCHAR(255) NOT NULL,
    description           TEXT,
    manager_id            UUID NOT NULL,
    approver_id           UUID,
    status                VARCHAR(32) NOT NULL DEFAULT 'planning',
    release_window_start  TIMESTAMPTZ NOT NULL,
    release_window_end    TIMESTAMPTZ NOT NULL,
    schedule_execute_time TIMESTAMPTZ,
    approval_config       JSONB NOT NULL DEFAULT '{}',
    created_by            UUID NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at            TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS product.release_plan_jobs (
    id               UUID PRIMARY KEY,
    release_plan_id  UUID NOT NULL REFERENCES product.release_plans(id) ON DELETE CASCADE,
    sort_order       INT NOT NULL DEFAULT 0,
    name             VARCHAR(255) NOT NULL,
    type             VARCHAR(16) NOT NULL,
    owner_id         UUID NOT NULL,
    status           VARCHAR(16) NOT NULL DEFAULT 'todo',
    text_content     TEXT,
    text_checked     BOOLEAN NOT NULL DEFAULT FALSE,
    workflow_ref     JSONB,
    execution_ref    JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS product.release_plan_audit_logs (
    id               UUID PRIMARY KEY,
    release_plan_id  UUID NOT NULL REFERENCES product.release_plans(id) ON DELETE CASCADE,
    actor_id         UUID NOT NULL,
    action           VARCHAR(64) NOT NULL,
    detail           JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_release_plans_product ON product.release_plans(product_id);
CREATE INDEX IF NOT EXISTS idx_release_plans_status ON product.release_plans(status);
CREATE INDEX IF NOT EXISTS idx_release_plan_jobs_plan ON product.release_plan_jobs(release_plan_id);

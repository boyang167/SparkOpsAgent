CREATE TABLE IF NOT EXISTS product.work_item_executions (
    id              UUID PRIMARY KEY,
    work_item_id    UUID NOT NULL REFERENCES product.work_items(id) ON DELETE CASCADE,
    stage_id        UUID NOT NULL,
    workflow_id     UUID NOT NULL,
    workflow_run_id UUID NOT NULL,
    triggered_by    UUID NOT NULL,
    status          VARCHAR(32) NOT NULL DEFAULT 'running',
    artifacts       JSONB NOT NULL DEFAULT '{}',
    error_message   TEXT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_work_item_executions_run
    ON product.work_item_executions(workflow_run_id);

CREATE INDEX IF NOT EXISTS idx_work_item_executions_item
    ON product.work_item_executions(work_item_id, started_at DESC);

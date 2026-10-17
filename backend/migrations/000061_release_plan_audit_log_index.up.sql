-- Keep the newest audit entries for one release plan index-only ordered.
CREATE INDEX IF NOT EXISTS idx_release_plan_audit_logs_plan_created_at
    ON product.release_plan_audit_logs(release_plan_id, created_at DESC);

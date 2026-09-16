-- M9: migrate legacy product_requirements into release_plans (idempotent)

INSERT INTO product.release_plans (
    id, product_id, product_plan_id, name, description, manager_id, status,
    release_window_start, release_window_end, approval_config, created_by, created_at, updated_at
)
SELECT
    r.id,
    r.product_id,
    r.product_plan_id,
    COALESCE(NULLIF(TRIM(r.project_name), ''), '迁移发布') || ' 发布',
    COALESCE(r.description, ''),
    '00000000-0000-4000-8000-000000000099'::uuid,
    CASE r.status
        WHEN 'pending'   THEN 'planning'
        WHEN 'approved'  THEN 'executing'
        WHEN 'rejected'  THEN 'denied'
        WHEN 'running'   THEN 'executing'
        WHEN 'completed' THEN 'success'
        ELSE 'planning'
    END,
    r.created_at,
    r.created_at + INTERVAL '7 days',
    '{"enabled": false}'::jsonb,
    '00000000-0000-4000-8000-000000000099'::uuid,
    r.created_at,
    COALESCE(r.updated_at, r.created_at)
FROM product.product_requirements r
WHERE r.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM product.release_plans rp WHERE rp.id = r.id);

INSERT INTO product.release_plan_jobs (
    id, release_plan_id, sort_order, name, type, owner_id, status,
    text_content, workflow_ref, execution_ref, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    r.id,
    0,
    COALESCE(NULLIF(TRIM(r.project_name), ''), '发布任务'),
    'text',
    '00000000-0000-4000-8000-000000000099'::uuid,
    CASE r.status
        WHEN 'completed' THEN 'done'
        WHEN 'rejected'  THEN 'skipped'
        WHEN 'running'   THEN 'running'
        ELSE 'todo'
    END,
    TRIM(BOTH E'\n' FROM CONCAT(
        COALESCE(r.description, ''),
        CASE WHEN COALESCE(r.project_tag, '') <> '' THEN E'\n分支: ' || r.project_tag ELSE '' END,
        CASE WHEN r.project_id IS NOT NULL THEN E'\nproject_id: ' || r.project_id::text ELSE '' END,
        CASE WHEN r.build_id IS NOT NULL THEN E'\nbuild_id: ' || r.build_id::text ELSE '' END
    )),
    NULL,
    CASE WHEN r.build_id IS NOT NULL THEN jsonb_build_object('build_id', r.build_id::text) ELSE NULL END,
    r.created_at,
    COALESCE(r.updated_at, r.created_at)
FROM product.product_requirements r
WHERE r.deleted_at IS NULL
  AND EXISTS (SELECT 1 FROM product.release_plans rp WHERE rp.id = r.id)
  AND NOT EXISTS (SELECT 1 FROM product.release_plan_jobs j WHERE j.release_plan_id = r.id);

INSERT INTO product.release_plan_audit_logs (id, release_plan_id, actor_id, action, detail, created_at)
SELECT
    gen_random_uuid(),
    r.id,
    '00000000-0000-4000-8000-000000000099'::uuid,
    'migrated_from_requirement',
    jsonb_build_object('requirement_id', r.id::text, 'legacy_status', r.status),
    NOW()
FROM product.product_requirements r
WHERE r.deleted_at IS NULL
  AND EXISTS (SELECT 1 FROM product.release_plans rp WHERE rp.id = r.id)
  AND NOT EXISTS (
      SELECT 1 FROM product.release_plan_audit_logs l
      WHERE l.release_plan_id = r.id AND l.action = 'migrated_from_requirement'
  );

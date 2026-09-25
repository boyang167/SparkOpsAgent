-- S6: migrate legacy product_plans into sprints (idempotent)

CREATE EXTENSION IF NOT EXISTS pgcrypto;

INSERT INTO product.sprint_processes (
    id, product_id, name, version, is_default, stages, created_by, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    p.id,
    '标准敏捷流程',
    1,
    TRUE,
    '[
      {"id": "00000000-0000-4000-8000-000000000011", "key": "dev", "name": "开发", "sort_order": 0, "color": "#3b82f6", "workflows": []},
      {"id": "00000000-0000-4000-8000-000000000012", "key": "test", "name": "测试", "sort_order": 1, "color": "#8b5cf6", "workflows": []},
      {"id": "00000000-0000-4000-8000-000000000013", "key": "uat", "name": "预发", "sort_order": 2, "color": "#f59e0b", "workflows": []},
      {"id": "00000000-0000-4000-8000-000000000014", "key": "release", "name": "发布", "sort_order": 3, "color": "#10b981", "workflows": []}
    ]'::jsonb,
    '00000000-0000-4000-8000-000000000099'::uuid,
    NOW(),
    NOW()
FROM product.products p
WHERE EXISTS (
    SELECT 1 FROM product.product_plans pp
    WHERE pp.product_id = p.id AND pp.deleted_at IS NULL
)
AND NOT EXISTS (
    SELECT 1 FROM product.sprint_processes sp
    WHERE sp.product_id = p.id AND sp.is_default = TRUE
);

INSERT INTO product.sprints (
    id, product_id, process_id, process_version, stages, name, goal, status,
    legacy_plan_id, created_by, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    pp.product_id,
    sp.id,
    sp.version,
    sp.stages,
    COALESCE(NULLIF(TRIM(pp.name), ''), pp.version, '迁移迭代'),
    COALESCE(pp.description, ''),
    CASE pp.status
        WHEN 'active'    THEN 'active'
        WHEN 'completed' THEN 'completed'
        ELSE 'planning'
    END,
    pp.id,
    '00000000-0000-4000-8000-000000000099'::uuid,
    COALESCE(pp.created_at, NOW()),
    COALESCE(pp.updated_at, pp.created_at, NOW())
FROM product.product_plans pp
INNER JOIN product.sprint_processes sp ON sp.product_id = pp.product_id AND sp.is_default = TRUE
WHERE pp.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM product.sprints s WHERE s.legacy_plan_id = pp.id
  );

UPDATE product.release_plans rp
SET sprint_id = s.id, updated_at = NOW()
FROM product.sprints s
WHERE rp.product_plan_id = s.legacy_plan_id
  AND rp.sprint_id IS NULL;

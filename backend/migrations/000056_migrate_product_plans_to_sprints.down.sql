-- S6 rollback: unlink release plans from migrated sprints (does not delete sprints)

UPDATE product.release_plans rp
SET sprint_id = NULL, updated_at = NOW()
FROM product.sprints s
WHERE rp.sprint_id = s.id
  AND s.legacy_plan_id IS NOT NULL;

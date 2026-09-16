DELETE FROM product.release_plan_audit_logs
WHERE action = 'migrated_from_requirement'
  AND release_plan_id IN (SELECT id FROM product.product_requirements);

DELETE FROM product.release_plan_jobs
WHERE release_plan_id IN (SELECT id FROM product.product_requirements);

DELETE FROM product.release_plans
WHERE id IN (SELECT id FROM product.product_requirements);

ALTER TABLE product.release_plans
  DROP COLUMN IF EXISTS hook_state,
  DROP COLUMN IF EXISTS hook_settings;

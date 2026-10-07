ALTER TABLE product.release_plans
  ADD COLUMN IF NOT EXISTS hook_settings JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS hook_state JSONB;

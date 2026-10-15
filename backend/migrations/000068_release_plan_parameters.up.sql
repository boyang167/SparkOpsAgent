ALTER TABLE product.release_plans
  ADD COLUMN IF NOT EXISTS parameters JSONB NOT NULL DEFAULT '{}';

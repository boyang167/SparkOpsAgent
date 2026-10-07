DROP TABLE IF EXISTS product.release_plan_versions;

ALTER TABLE product.release_plans DROP COLUMN IF EXISTS version;

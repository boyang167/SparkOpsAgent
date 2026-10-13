ALTER TABLE product.release_plan_jobs
    ADD COLUMN IF NOT EXISTS text_format VARCHAR(16) NOT NULL DEFAULT '';

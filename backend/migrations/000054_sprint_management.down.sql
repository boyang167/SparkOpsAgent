DROP INDEX IF EXISTS product.idx_release_plans_sprint;
ALTER TABLE product.release_plans DROP COLUMN IF EXISTS sprint_id;

DROP INDEX IF EXISTS product.idx_work_items_board;
DROP TABLE IF EXISTS product.work_items;

DROP INDEX IF EXISTS product.idx_sprints_product_status;
DROP TABLE IF EXISTS product.sprints;

DROP INDEX IF EXISTS product.idx_sprint_process_product_default;
DROP TABLE IF EXISTS product.sprint_processes;

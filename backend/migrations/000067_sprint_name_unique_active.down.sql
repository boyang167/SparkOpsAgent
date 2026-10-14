DROP INDEX IF EXISTS product.idx_sprints_product_name_active;

ALTER TABLE product.sprints ADD CONSTRAINT sprints_product_id_name_key UNIQUE (product_id, name);

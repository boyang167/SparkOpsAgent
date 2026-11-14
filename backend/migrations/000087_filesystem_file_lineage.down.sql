DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM filesystem.file_lineage) THEN
        RAISE EXCEPTION 'cannot roll back active filesystem file lineage';
    END IF;
END
$$;

DROP TABLE filesystem.file_lineage;

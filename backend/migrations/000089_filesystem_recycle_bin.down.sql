DROP INDEX IF EXISTS filesystem.ix_filesystem_nodes_trash_entries;

ALTER TABLE filesystem.nodes
    DROP COLUMN IF EXISTS deleted_by;

DROP INDEX IF EXISTS filesystem.ux_filesystem_nodes_root_name;
DROP INDEX IF EXISTS filesystem.ux_filesystem_nodes_child_name;

CREATE UNIQUE INDEX ux_filesystem_nodes_root_name
    ON filesystem.nodes(namespace_id, normalized_name)
    WHERE parent_id IS NULL;

CREATE UNIQUE INDEX ux_filesystem_nodes_child_name
    ON filesystem.nodes(namespace_id, parent_id, normalized_name)
    WHERE parent_id IS NOT NULL;
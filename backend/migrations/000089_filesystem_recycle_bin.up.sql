-- Allow active nodes to reuse names held by trashed nodes; support recycle-bin metadata.
DROP INDEX IF EXISTS filesystem.ux_filesystem_nodes_root_name;
DROP INDEX IF EXISTS filesystem.ux_filesystem_nodes_child_name;

CREATE UNIQUE INDEX ux_filesystem_nodes_root_name
    ON filesystem.nodes(namespace_id, normalized_name)
    WHERE parent_id IS NULL AND state = 'ACTIVE';

CREATE UNIQUE INDEX ux_filesystem_nodes_child_name
    ON filesystem.nodes(namespace_id, parent_id, normalized_name)
    WHERE parent_id IS NOT NULL AND state = 'ACTIVE';

ALTER TABLE filesystem.nodes
    ADD COLUMN IF NOT EXISTS deleted_by UUID;

CREATE INDEX IF NOT EXISTS ix_filesystem_nodes_trash_entries
    ON filesystem.nodes(namespace_id, deleted_at DESC, id DESC)
    WHERE state = 'TRASHED';
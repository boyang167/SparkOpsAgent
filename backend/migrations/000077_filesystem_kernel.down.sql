ALTER TABLE IF EXISTS filesystem.namespaces
    DROP CONSTRAINT IF EXISTS fk_namespaces_root_node;

ALTER TABLE IF EXISTS filesystem.nodes
    DROP CONSTRAINT IF EXISTS fk_nodes_parent_namespace;

ALTER TABLE IF EXISTS filesystem.nodes
    DROP CONSTRAINT IF EXISTS fk_nodes_current_revision;

DROP TABLE IF EXISTS filesystem.file_links;
DROP TABLE IF EXISTS filesystem.command_dedup;
DROP TABLE IF EXISTS filesystem.resource_grants;
DROP TABLE IF EXISTS filesystem.quota_reservations;
DROP TABLE IF EXISTS filesystem.quota_accounts;
DROP TABLE IF EXISTS filesystem.storage_replicas;
DROP TABLE IF EXISTS filesystem.file_revisions;
DROP TABLE IF EXISTS filesystem.blobs;
DROP TABLE IF EXISTS filesystem.nodes;
DROP TABLE IF EXISTS filesystem.namespaces;

DROP SCHEMA IF EXISTS filesystem;

CREATE UNIQUE INDEX IF NOT EXISTS ux_filesystem_namespaces_id_organization
    ON filesystem.namespaces(id, organization_id);

CREATE TABLE filesystem.file_lineage (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    source_namespace_id UUID NOT NULL,
    source_node_id UUID NOT NULL,
    source_revision_id UUID NOT NULL,
    target_namespace_id UUID NOT NULL,
    target_node_id UUID NOT NULL UNIQUE,
    target_revision_id UUID NOT NULL,
    relation VARCHAR(64) NOT NULL CHECK (relation = 'ADOPTED_AS_ORGANIZATION_FILE'),
    command_id UUID NOT NULL UNIQUE,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_file_lineage_distinct_nodes CHECK (source_node_id <> target_node_id),
    CONSTRAINT fk_file_lineage_source_namespace_organization
        FOREIGN KEY (source_namespace_id, organization_id)
        REFERENCES filesystem.namespaces(id, organization_id),
    CONSTRAINT fk_file_lineage_target_namespace_organization
        FOREIGN KEY (target_namespace_id, organization_id)
        REFERENCES filesystem.namespaces(id, organization_id),
    CONSTRAINT fk_file_lineage_source_node_namespace
        FOREIGN KEY (source_node_id, source_namespace_id)
        REFERENCES filesystem.nodes(id, namespace_id),
    CONSTRAINT fk_file_lineage_target_node_namespace
        FOREIGN KEY (target_node_id, target_namespace_id)
        REFERENCES filesystem.nodes(id, namespace_id),
    CONSTRAINT fk_file_lineage_source_revision_node
        FOREIGN KEY (source_revision_id, source_node_id)
        REFERENCES filesystem.file_revisions(id, node_id),
    CONSTRAINT fk_file_lineage_target_revision_node
        FOREIGN KEY (target_revision_id, target_node_id)
        REFERENCES filesystem.file_revisions(id, node_id)
);

CREATE INDEX ix_file_lineage_source
    ON filesystem.file_lineage(organization_id, source_node_id, created_at DESC);

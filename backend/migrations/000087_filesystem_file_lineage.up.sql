CREATE TABLE filesystem.file_lineage (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    source_namespace_id UUID NOT NULL REFERENCES filesystem.namespaces(id),
    source_node_id UUID NOT NULL REFERENCES filesystem.nodes(id),
    source_revision_id UUID NOT NULL REFERENCES filesystem.file_revisions(id),
    target_namespace_id UUID NOT NULL REFERENCES filesystem.namespaces(id),
    target_node_id UUID NOT NULL UNIQUE REFERENCES filesystem.nodes(id),
    target_revision_id UUID NOT NULL REFERENCES filesystem.file_revisions(id),
    relation VARCHAR(64) NOT NULL CHECK (relation = 'ADOPTED_AS_ORGANIZATION_FILE'),
    command_id UUID NOT NULL UNIQUE,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_file_lineage_distinct_nodes CHECK (source_node_id <> target_node_id)
);

CREATE INDEX ix_file_lineage_source
    ON filesystem.file_lineage(organization_id, source_node_id, created_at DESC);

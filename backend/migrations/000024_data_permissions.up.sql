CREATE TABLE IF NOT EXISTS governance.data_resources (
    id               UUID PRIMARY KEY,
    resource_key     VARCHAR(64) NOT NULL,
    schema_name      VARCHAR(64) NOT NULL,
    table_name       VARCHAR(128) NOT NULL,
    owner_column     VARCHAR(64) NOT NULL DEFAULT 'created_by',
    workspace_column VARCHAR(64) NOT NULL DEFAULT 'workspace_id',
    title            VARCHAR(128) NOT NULL DEFAULT '',
    description      VARCHAR(512) NOT NULL DEFAULT '',
    enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(resource_key)
);

CREATE TABLE IF NOT EXISTS governance.data_fields (
    id              UUID PRIMARY KEY,
    resource_id     UUID NOT NULL REFERENCES governance.data_resources(id) ON DELETE CASCADE,
    column_name     VARCHAR(64) NOT NULL,
    title           VARCHAR(128) NOT NULL DEFAULT '',
    sensitive_level VARCHAR(16) NOT NULL DEFAULT 'internal',
    mask_strategy   VARCHAR(32) NOT NULL DEFAULT 'omit',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(resource_id, column_name)
);

CREATE TABLE IF NOT EXISTS governance.role_data_policies (
    role_id         UUID NOT NULL REFERENCES governance.roles(id) ON DELETE CASCADE,
    resource_key    VARCHAR(64) NOT NULL,
    action          VARCHAR(16) NOT NULL,
    data_scope      VARCHAR(32) NOT NULL,
    custom_rules    JSONB NOT NULL DEFAULT '[]',
    allowed_columns JSONB NULL,
    denied_columns  JSONB NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(role_id, resource_key, action)
);

CREATE INDEX IF NOT EXISTS idx_role_data_policies_resource
    ON governance.role_data_policies(resource_key);

INSERT INTO governance.data_resources
    (id, resource_key, schema_name, table_name, owner_column, workspace_column, title, description)
VALUES
    ('a1000001-0000-4000-8000-000000000001', 'project', 'workspace_app', 'projects', 'created_by', 'workspace_id', '项目', '工作区项目数据'),
    ('a1000001-0000-4000-8000-000000000002', 'agent', 'agent', 'agents', 'created_by', 'workspace_id', 'Agent', '工作区 Agent 数据'),
    ('a1000001-0000-4000-8000-000000000003', 'member', 'governance', 'role_bindings', 'member_id', 'workspace_id', '成员绑定', '工作区成员角色绑定')
ON CONFLICT (resource_key) DO NOTHING;

-- Remove workspaces whose organization is missing or soft-deleted, plus scoped data.
-- Equivalent to: go run ./cmd/cleanup-orphan-workspaces

CREATE TEMP TABLE tmp_orphan_ws ON COMMIT DROP AS
SELECT w.id
FROM tenant.workspaces w
LEFT JOIN tenant.organizations o ON o.id = w.org_id AND o.deleted_at IS NULL
WHERE w.deleted_at IS NULL AND o.id IS NULL;

DELETE FROM agent_runtime.messages
WHERE conversation_id IN (
    SELECT c.id FROM agent_runtime.conversations c
    WHERE c.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM agent_runtime.runs
WHERE conversation_id IN (
    SELECT c.id FROM agent_runtime.conversations c
    WHERE c.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM agent_runtime.conversations
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM agent.agent_versions
WHERE agent_id IN (
    SELECT a.id FROM agent.agents a
    WHERE a.project_id IN (
        SELECT p.id FROM workspace_app.projects p
        WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
    )
);

DELETE FROM agent.agents
WHERE project_id IN (
    SELECT p.id FROM workspace_app.projects p
    WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM workflow.workflow_runs
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)
   OR workflow_id IN (
        SELECT wf.id FROM workflow.workflows wf
        WHERE wf.project_id IN (
            SELECT p.id FROM workspace_app.projects p
            WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
        )
   );

DELETE FROM workflow.workflows
WHERE project_id IN (
    SELECT p.id FROM workspace_app.projects p
    WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM delivery.cronjobs
WHERE project_id IN (
    SELECT p.id FROM workspace_app.projects p
    WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM delivery.builds
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)
   OR project_id IN (
        SELECT p.id FROM workspace_app.projects p
        WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
   );

DELETE FROM product.product_requirements
WHERE product_id IN (
    SELECT pr.id FROM product.products pr
    WHERE pr.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM product.product_plans
WHERE product_id IN (
    SELECT pr.id FROM product.products pr
    WHERE pr.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM product.products
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM knowledge.documents
WHERE knowledge_base_id IN (
    SELECT kb.id FROM knowledge.knowledge_bases kb
    WHERE kb.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM knowledge.knowledge_bases
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM notification.notifications
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM workspace_app.project_members
WHERE project_id IN (
    SELECT p.id FROM workspace_app.projects p
    WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM workspace_app.projects
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM workspace_app.secrets
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM workspace_app.templates
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM workspace_app.dockerfiles
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM governance.role_menu_buttons
WHERE role_id IN (
    SELECT r.id FROM governance.roles r
    WHERE r.workspace_id IN (SELECT id FROM tmp_orphan_ws)
)
   OR button_id IN (
        SELECT mb.id FROM governance.menu_buttons mb
        WHERE mb.menu_id IN (
            SELECT m.id FROM governance.menus m
            WHERE m.workspace_id IN (SELECT id FROM tmp_orphan_ws)
        )
   );

DELETE FROM governance.menu_buttons
WHERE menu_id IN (
    SELECT m.id FROM governance.menus m
    WHERE m.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM governance.menu_params
WHERE menu_id IN (
    SELECT m.id FROM governance.menus m
    WHERE m.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM governance.menus
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM governance.role_api_policies
WHERE role_id IN (
    SELECT r.id FROM governance.roles r
    WHERE r.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM governance.role_data_policies
WHERE role_id IN (
    SELECT r.id FROM governance.roles r
    WHERE r.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM governance.role_bindings
WHERE role_id IN (
    SELECT r.id FROM governance.roles r
    WHERE r.workspace_id IN (SELECT id FROM tmp_orphan_ws)
);

DELETE FROM governance.casbin_rule
WHERE v1 IN (SELECT id::text FROM tmp_orphan_ws);

DELETE FROM governance.roles
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM platform_admin.statistics_cache
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM platform_admin.sys_cfg
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM platform.consul_kv
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM identity.api_tokens
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM tenant.workspace_members
WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws);

DELETE FROM tenant.workspaces
WHERE id IN (SELECT id FROM tmp_orphan_ws);

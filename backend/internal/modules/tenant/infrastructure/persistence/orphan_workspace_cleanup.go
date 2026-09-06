package persistence

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type OrphanWorkspaceRow struct {
	ID    string
	OrgID string
	Name  string
	Slug  string
}

// ListOrphanWorkspaces returns workspaces whose org is missing or soft-deleted.
func (s *TenantStore) ListOrphanWorkspaces(ctx context.Context) ([]OrphanWorkspaceRow, error) {
	var rows []OrphanWorkspaceRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT w.id, w.org_id, w.name, w.slug
		FROM tenant.workspaces w
		LEFT JOIN tenant.organizations o ON o.id = w.org_id AND o.deleted_at IS NULL
		WHERE w.deleted_at IS NULL AND o.id IS NULL
		ORDER BY w.name ASC
	`).Scan(&rows).Error
	return rows, err
}

// PurgeOrphanWorkspaces hard-deletes workspaces with no active organization and their scoped data.
func (s *TenantStore) PurgeOrphanWorkspaces(ctx context.Context) (int, error) {
	orphans, err := s.ListOrphanWorkspaces(ctx)
	if err != nil {
		return 0, err
	}
	if len(orphans) == 0 {
		return 0, nil
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			CREATE TEMP TABLE tmp_orphan_ws ON COMMIT DROP AS
			SELECT w.id
			FROM tenant.workspaces w
			LEFT JOIN tenant.organizations o ON o.id = w.org_id AND o.deleted_at IS NULL
			WHERE w.deleted_at IS NULL AND o.id IS NULL
		`).Error; err != nil {
			return fmt.Errorf("create temp orphan workspaces: %w", err)
		}

		steps := []struct {
			label string
			sql   string
		}{
			{"agent_runtime.messages", `
				DELETE FROM agent_runtime.messages
				WHERE conversation_id IN (
					SELECT c.id FROM agent_runtime.conversations c
					WHERE c.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"agent_runtime.runs", `
				DELETE FROM agent_runtime.runs
				WHERE conversation_id IN (
					SELECT c.id FROM agent_runtime.conversations c
					WHERE c.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"agent_runtime.conversations", `
				DELETE FROM agent_runtime.conversations
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"agent.agent_versions", `
				DELETE FROM agent.agent_versions
				WHERE agent_id IN (
					SELECT a.id FROM agent.agents a
					WHERE a.project_id IN (
						SELECT p.id FROM workspace_app.projects p
						WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
					)
				)`},
			{"agent.agents", `
				DELETE FROM agent.agents
				WHERE project_id IN (
					SELECT p.id FROM workspace_app.projects p
					WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"workflow.workflow_runs", `
				DELETE FROM workflow.workflow_runs
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)
				   OR workflow_id IN (
						SELECT wf.id FROM workflow.workflows wf
						WHERE wf.project_id IN (
							SELECT p.id FROM workspace_app.projects p
							WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
						)
				   )`},
			{"workflow.workflows", `
				DELETE FROM workflow.workflows
				WHERE project_id IN (
					SELECT p.id FROM workspace_app.projects p
					WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"delivery.cronjobs", `
				DELETE FROM delivery.cronjobs
				WHERE project_id IN (
					SELECT p.id FROM workspace_app.projects p
					WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"delivery.builds", `
				DELETE FROM delivery.builds
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)
				   OR project_id IN (
						SELECT p.id FROM workspace_app.projects p
						WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				   )`},
			{"product.product_requirements", `
				DELETE FROM product.product_requirements
				WHERE product_id IN (
					SELECT pr.id FROM product.products pr
					WHERE pr.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"product.product_plans", `
				DELETE FROM product.product_plans
				WHERE product_id IN (
					SELECT pr.id FROM product.products pr
					WHERE pr.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"product.products", `
				DELETE FROM product.products
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"knowledge.documents", `
				DELETE FROM knowledge.documents
				WHERE knowledge_base_id IN (
					SELECT kb.id FROM knowledge.knowledge_bases kb
					WHERE kb.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"knowledge.knowledge_bases", `
				DELETE FROM knowledge.knowledge_bases
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"notification.notifications", `
				DELETE FROM notification.notifications
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"workspace_app.project_members", `
				DELETE FROM workspace_app.project_members
				WHERE project_id IN (
					SELECT p.id FROM workspace_app.projects p
					WHERE p.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"workspace_app.projects", `
				DELETE FROM workspace_app.projects
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"workspace_app.secrets", `
				DELETE FROM workspace_app.secrets
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"workspace_app.templates", `
				DELETE FROM workspace_app.templates
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"workspace_app.dockerfiles", `
				DELETE FROM workspace_app.dockerfiles
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"governance.role_menu_buttons", `
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
				   )`},
			{"governance.menu_buttons", `
				DELETE FROM governance.menu_buttons
				WHERE menu_id IN (
					SELECT m.id FROM governance.menus m
					WHERE m.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"governance.menu_params", `
				DELETE FROM governance.menu_params
				WHERE menu_id IN (
					SELECT m.id FROM governance.menus m
					WHERE m.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"governance.menus", `
				DELETE FROM governance.menus
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"governance.role_api_policies", `
				DELETE FROM governance.role_api_policies
				WHERE role_id IN (
					SELECT r.id FROM governance.roles r
					WHERE r.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"governance.role_data_policies", `
				DELETE FROM governance.role_data_policies
				WHERE role_id IN (
					SELECT r.id FROM governance.roles r
					WHERE r.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"governance.role_bindings", `
				DELETE FROM governance.role_bindings
				WHERE role_id IN (
					SELECT r.id FROM governance.roles r
					WHERE r.workspace_id IN (SELECT id FROM tmp_orphan_ws)
				)`},
			{"governance.casbin_rule", `
				DELETE FROM governance.casbin_rule
				WHERE v1 IN (SELECT id::text FROM tmp_orphan_ws)`},
			{"governance.roles", `
				DELETE FROM governance.roles
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"platform_admin.statistics_cache", `
				DELETE FROM platform_admin.statistics_cache
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"platform_admin.sys_cfg", `
				DELETE FROM platform_admin.sys_cfg
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"identity.api_tokens", `
				DELETE FROM identity.api_tokens
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"tenant.workspace_members", `
				DELETE FROM tenant.workspace_members
				WHERE workspace_id IN (SELECT id FROM tmp_orphan_ws)`},
			{"tenant.workspaces", `
				DELETE FROM tenant.workspaces
				WHERE id IN (SELECT id FROM tmp_orphan_ws)`},
		}

		for _, step := range steps {
			if err := tx.Exec("SAVEPOINT purge_step").Error; err != nil {
				return fmt.Errorf("savepoint before %s: %w", step.label, err)
			}
			if err := tx.Exec(step.sql).Error; err != nil {
				if isMissingRelation(err) {
					if rbErr := tx.Exec("ROLLBACK TO SAVEPOINT purge_step").Error; rbErr != nil {
						return fmt.Errorf("rollback optional %s: %w", step.label, rbErr)
					}
					continue
				}
				return fmt.Errorf("purge %s: %w", step.label, err)
			}
			if err := tx.Exec("RELEASE SAVEPOINT purge_step").Error; err != nil {
				return fmt.Errorf("release savepoint after %s: %w", step.label, err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(orphans), nil
}

func isMissingRelation(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "does not exist") && strings.Contains(msg, "relation")
}

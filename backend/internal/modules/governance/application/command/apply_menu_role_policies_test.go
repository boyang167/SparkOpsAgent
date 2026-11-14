package command_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDefaultMenuRolePolicies_includeAdminOnRestrictedPaths(t *testing.T) {
	restricted := []string{
		"/config/secrets",
		"/publish/products",
		"/publish/release-plans",
		"/config/webhooks",
		"/plugins/kubernetes/access-controls",
	}
	for _, path := range restricted {
		roles, ok := command.DefaultMenuRolePolicies[path]
		require.Truef(t, ok, "missing policy for %s", path)
		require.Containsf(t, roles, "admin", "%s must include admin", path)
		require.Containsf(t, roles, "operator", "%s must include operator", path)
	}
}

func TestDefaultMenuRolePolicies_templatesHiddenFromViewer(t *testing.T) {
	roles := command.DefaultMenuRolePolicies["/templates"]
	require.NotContains(t, roles, "viewer")
	require.Contains(t, roles, "developer")
}

func TestDefaultMenuRolePolicies_filesRoles(t *testing.T) {
	require.Equal(t, []string{"admin", "developer", "operator"},
		command.DefaultMenuRolePolicies["/files"])
}

func TestApplyMenuRolePolicies_preservesConfiguredFilesRoles(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS governance").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE governance.menus (
			id TEXT PRIMARY KEY, workspace_id TEXT, parent_id TEXT, name TEXT,
			path TEXT, title TEXT, icon TEXT, component TEXT, hidden BOOLEAN,
			sort INTEGER, keep_alive BOOLEAN, active_menu TEXT, close_tab BOOLEAN,
			is_basic BOOLEAN, transition_animation TEXT, created_at DATETIME,
			updated_at DATETIME, deleted_at DATETIME
		);
		CREATE TABLE governance.role_menus (role_id TEXT, menu_id TEXT);
	`).Error)

	ctx := context.Background()
	workspaceID := kernel.NewWorkspaceID()
	menuStore := persistence.NewMenuStore(db)
	files := &model.Menu{
		ID: "menu-files", WorkspaceID: workspaceID, Name: "files",
		Path: "/files", Title: "文件中心", Component: "/files",
	}
	require.NoError(t, menuStore.Save(ctx, files))
	require.NoError(t, menuStore.SetRoleIDs(ctx, files.ID, []string{"custom-role"}))

	roles := newMemoryRoleRepo()
	for _, name := range []string{"admin", "developer", "operator"} {
		require.NoError(t, roles.Save(ctx, model.NewRole(workspaceID, name, []string{"file:read"}, nil)))
	}
	handler := command.NewApplyMenuRolePoliciesHandler(menuStore, roles, nil)
	require.NoError(t, handler.Handle(ctx, workspaceID, command.ApplyMenuRolePoliciesOptions{
		SkipIfConfigured: true,
	}))

	roleIDs, err := menuStore.ListRoleIDs(ctx, files.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"custom-role"}, roleIDs)
}

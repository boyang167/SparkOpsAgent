package entitlements

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAPIPathEntitlementMapsPluginBusinessRoute(t *testing.T) {
	require.Equal(t, "plugin:example", APIPathEntitlement("/api/v1/plugins/example/ping"))
}

func TestAPIPathEntitlementSkipsOnlyWorkspacePluginManagementRoutes(t *testing.T) {
	require.Empty(t, APIPathEntitlement("/api/v1/workspaces/ws/plugins"))
	require.Empty(t, APIPathEntitlement("/api/v1/workspaces/ws/plugins/example/enable"))
	require.Empty(t, APIPathEntitlement("/api/v1/workspaces/ws/plugins/example/disable"))
	require.Equal(t, "plugin:example", APIPathEntitlement("/api/v1/plugins/example/workspaces/ws"))
}

func TestAllowsModuleRequiresPluginInWorkspaceEntitlements(t *testing.T) {
	ents := WorkspaceEntitlements{Plugins: []string{"other"}}
	require.False(t, ents.AllowsModule("plugin:example"))

	ents.Plugins = append(ents.Plugins, "example")
	require.True(t, ents.AllowsModule("plugin:example"))
}

func TestPathEntitlementMapsPluginMenuRoute(t *testing.T) {
	require.Equal(t, "plugin:example", PathEntitlement("/plugins/example/settings"))
	require.Empty(t, PathEntitlement("/plugins"))
}

func TestResolveForWorkspaceLoadsOnlyEnabledPluginBindings(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform_admin").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS plugin").Error)
	require.NoError(t, db.Exec(`CREATE TABLE platform_admin.sys_cfg (
		config TEXT, deleted_at DATETIME, created_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE platform_admin.cluster_info (
		id TEXT, deleted_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE plugin.workspace_bindings (
		workspace_id TEXT, plugin_id TEXT, enabled BOOLEAN
	)`).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO plugin.workspace_bindings (workspace_id, plugin_id, enabled) VALUES
		 (?, 'enabled-plugin', 1), (?, 'disabled-plugin', 0), ('other-workspace', 'other-plugin', 1)`,
		"workspace-1", "workspace-1",
	).Error)

	ents, err := ResolveForWorkspace(context.Background(), db, "workspace-1")

	require.NoError(t, err)
	require.Equal(t, []string{"enabled-plugin"}, ents.Plugins)
}

package acl_test

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const binderManifest = `id: example
version: "0.1.0"
display_name: Example Plugin
route_prefix: /api/v1/plugins/example
entitlement: plugin:example
menus:
  - title: Example
    path: /plugins/example
    component: plugins/example/page
apis:
  - method: GET
    path: /api/v1/plugins/example/ping
frontend:
  pages:
    - app/(platform)/plugins/example/page.tsx
`

const kubernetesBinderManifest = `id: kubernetes
version: "0.1.0"
display_name: Kubernetes Plugin
route_prefix: /api/v1/plugins/kubernetes
entitlement: plugin:kubernetes
menus:
  - title: Pods
    path: /plugins/kubernetes/pods
    parent: /plugins/kubernetes
    component: plugins/kubernetes/pods/page
  - title: Kubernetes
    path: /plugins/kubernetes
    component: plugins/kubernetes/page
apis:
  - method: ANY
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/proxy/*
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/metrics
    read_only: true
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/custom/metrics
  - method: POST
    path: /api/v1/plugins/kubernetes/clusters/:clusterId/pods/:namespace/:pod/exec
frontend:
  pages:
    - app/(platform)/plugins/kubernetes/page.tsx
`

func TestGovernanceBinderBindsAndUnbindsPluginResources(t *testing.T) {
	db := openBinderDB(t)
	wsID := kernel.NewWorkspaceID()
	adminRoleID, viewerRoleID := kernel.NewRoleID().String(), kernel.NewRoleID().String()
	require.NoError(t, db.Exec(
		`INSERT INTO governance.roles (id, workspace_id, permissions) VALUES (?, ?, ?), (?, ?, ?)`,
		adminRoleID, wsID.String(), `["governance:admin"]`,
		viewerRoleID, wsID.String(), `["project:read"]`,
	).Error)
	spec, err := manifest.Parse([]byte(binderManifest))
	require.NoError(t, err)
	binder := acl.NewGovernanceBinder(db)

	require.NoError(t, binder.Bind(context.Background(), wsID, spec))

	require.Equal(t, int64(1), countRows(t, db, "governance.menus", "workspace_id = ? AND path = ?", wsID.String(), "/plugins/example"))
	require.Equal(t, int64(1), countRows(t, db, "governance.menus", "workspace_id = ? AND path = ? AND parent_id IS NULL", wsID.String(), "/plugins"))
	var childParent string
	require.NoError(t, db.Table("governance.menus").Select("parent_id").
		Where("workspace_id = ? AND path = ?", wsID.String(), "/plugins/example").
		Scan(&childParent).Error)
	require.NotEmpty(t, childParent)
	require.Equal(t, int64(1), countRows(t, db, "platform.apis", "path = ? AND method = ?", "/api/v1/plugins/example/ping", "GET"))
	require.Equal(t, int64(1), countRows(t, db, "governance.role_api_policies", "role_id = ? AND path = ?", adminRoleID, "/api/v1/plugins/example/ping"))
	require.Equal(t, int64(1), countRows(t, db, "governance.role_api_policies", "role_id = ? AND path = ? AND method = ?", viewerRoleID, "/api/v1/plugins/example/ping", "GET"))

	require.NoError(t, binder.Unbind(context.Background(), wsID, "example"))

	require.Zero(t, countRows(t, db, "governance.menus", "workspace_id = ? AND path LIKE ?", wsID.String(), "/plugins/example%"))
	require.Equal(t, int64(1), countRows(t, db, "governance.menus", "workspace_id = ? AND path = ?", wsID.String(), "/plugins"))
	require.Zero(t, countRows(t, db, "governance.role_api_policies", "role_id = ?", adminRoleID))
	require.Equal(t, int64(1), countRows(t, db, "platform.apis", "path = ? AND method = ?", "/api/v1/plugins/example/ping", "GET"))
}

func TestGovernanceBinderBindsNestedMenusAndMemberReadOnlyPluginAPIs(t *testing.T) {
	db := openBinderDB(t)
	wsID := kernel.NewWorkspaceID()
	adminRoleID, viewerRoleID := kernel.NewRoleID().String(), kernel.NewRoleID().String()
	require.NoError(t, db.Exec(
		`INSERT INTO governance.roles (id, workspace_id, permissions) VALUES (?, ?, ?), (?, ?, ?)`,
		adminRoleID, wsID.String(), `["governance:admin"]`,
		viewerRoleID, wsID.String(), `["project:read"]`,
	).Error)
	spec, err := manifest.Parse([]byte(kubernetesBinderManifest))
	require.NoError(t, err)
	binder := acl.NewGovernanceBinder(db)

	require.NoError(t, binder.Bind(context.Background(), wsID, spec))

	var rootID string
	require.NoError(t, db.Table("governance.menus").Select("id").
		Where("workspace_id = ? AND path = ?", wsID.String(), "/plugins/kubernetes").
		Scan(&rootID).Error)
	var podsParentID string
	require.NoError(t, db.Table("governance.menus").Select("parent_id").
		Where("workspace_id = ? AND path = ?", wsID.String(), "/plugins/kubernetes/pods").
		Scan(&podsParentID).Error)
	require.NotEmpty(t, rootID)
	require.Equal(t, rootID, podsParentID)

	proxyPolicyPath := "/api/v1/plugins/kubernetes/clusters/:clusterId/proxy/*"
	metricsPolicyPath := "/api/v1/plugins/kubernetes/clusters/:clusterId/metrics"
	customMetricsPolicyPath := "/api/v1/plugins/kubernetes/clusters/:clusterId/custom/metrics"
	execPolicyPath := "/api/v1/plugins/kubernetes/clusters/:clusterId/pods/:namespace/:pod/exec"
	for _, method := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"} {
		require.Equal(t, int64(1), countRows(t, db, "platform.apis", "path = ? AND method = ?", proxyPolicyPath, method))
	}
	require.Equal(t, int64(1), countRows(t, db, "platform.apis", "path = ? AND method = ?", metricsPolicyPath, "POST"))

	require.Equal(t, int64(1), countRows(t, db, "governance.role_api_policies", "role_id = ? AND path = ? AND method = ?", viewerRoleID, proxyPolicyPath, "GET"))
	require.Equal(t, int64(1), countRows(t, db, "governance.role_api_policies", "role_id = ? AND path = ? AND method = ?", viewerRoleID, proxyPolicyPath, "HEAD"))
	require.Zero(t, countRows(t, db, "governance.role_api_policies", "role_id = ? AND path = ? AND method = ?", viewerRoleID, proxyPolicyPath, "POST"))
	require.Equal(t, int64(1), countRows(t, db, "governance.role_api_policies", "role_id = ? AND path = ? AND method = ?", viewerRoleID, metricsPolicyPath, "POST"))
	require.Zero(t, countRows(t, db, "governance.role_api_policies", "role_id = ? AND path = ? AND method = ?", viewerRoleID, customMetricsPolicyPath, "POST"))
	require.Zero(t, countRows(t, db, "governance.role_api_policies", "role_id = ? AND path = ? AND method = ?", viewerRoleID, execPolicyPath, "POST"))

	require.Equal(t, int64(1), countRows(t, db, "governance.role_api_policies", "role_id = ? AND path = ? AND method = ?", adminRoleID, proxyPolicyPath, "POST"))
	require.Equal(t, int64(1), countRows(t, db, "governance.role_api_policies", "role_id = ? AND path = ? AND method = ?", adminRoleID, customMetricsPolicyPath, "POST"))
	require.Equal(t, int64(1), countRows(t, db, "governance.role_api_policies", "role_id = ? AND path = ? AND method = ?", adminRoleID, execPolicyPath, "POST"))

	require.NoError(t, binder.Unbind(context.Background(), wsID, "kubernetes"))

	require.Zero(t, countRows(t, db, "governance.role_api_policies", "path = ?", proxyPolicyPath))
	require.Zero(t, countRows(t, db, "governance.role_api_policies", "path = ?", metricsPolicyPath))
}

func TestUnitOfWorkRollsBackGovernanceWrites(t *testing.T) {
	db := openBinderDB(t)
	wsID := kernel.NewWorkspaceID()
	spec, err := manifest.Parse([]byte(binderManifest))
	require.NoError(t, err)
	uow := acl.NewUnitOfWork(db)
	expected := errors.New("stop transaction")

	err = uow.WithinTransaction(context.Background(), func(_ command.BindingRepository, binder command.WorkspaceBinder) error {
		require.NoError(t, binder.Bind(context.Background(), wsID, spec))
		return expected
	})

	require.ErrorIs(t, err, expected)
	require.Zero(t, countRows(t, db, "governance.menus", "workspace_id = ?", wsID.String()))
}

func openBinderDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	for _, schema := range []string{"plugin", "governance", "platform"} {
		require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS "+schema).Error)
	}
	statements := []string{
		`CREATE TABLE governance.menus (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, parent_id TEXT, name TEXT NOT NULL,
			path TEXT NOT NULL, title TEXT NOT NULL, icon TEXT NOT NULL DEFAULT '',
			component TEXT NOT NULL DEFAULT '', hidden BOOLEAN NOT NULL DEFAULT 0,
			sort INTEGER NOT NULL DEFAULT 0, keep_alive BOOLEAN NOT NULL DEFAULT 0,
			active_menu TEXT NOT NULL DEFAULT '', close_tab BOOLEAN NOT NULL DEFAULT 0,
			is_basic BOOLEAN NOT NULL DEFAULT 0, transition_animation TEXT NOT NULL DEFAULT 'global',
			created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME
		)`,
		`CREATE TABLE governance.roles (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, permissions BLOB NOT NULL
		)`,
		`CREATE TABLE governance.role_api_policies (
			role_id TEXT NOT NULL, path TEXT NOT NULL, method TEXT NOT NULL,
			PRIMARY KEY(role_id, path, method)
		)`,
		`CREATE TABLE platform.apis (
			id TEXT PRIMARY KEY, path TEXT NOT NULL, method TEXT NOT NULL,
			api_group TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
			UNIQUE(path, method)
		)`,
		`CREATE TABLE plugin.plugins (
			id TEXT PRIMARY KEY, version TEXT NOT NULL, display_name TEXT NOT NULL,
			manifest_yaml TEXT NOT NULL, package_digest TEXT NOT NULL DEFAULT '',
			installed_files TEXT NOT NULL DEFAULT '[]', status TEXT NOT NULL,
			pr_url TEXT NOT NULL DEFAULT '', created_by TEXT NOT NULL,
			created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE plugin.workspace_bindings (
			workspace_id TEXT NOT NULL, plugin_id TEXT NOT NULL, enabled BOOLEAN NOT NULL,
			enabled_by TEXT, enabled_at DATETIME, PRIMARY KEY(workspace_id, plugin_id)
		)`,
	}
	for _, statement := range statements {
		require.NoError(t, db.Exec(statement).Error)
	}
	return db
}

func countRows(t *testing.T, db *gorm.DB, table, where string, args ...any) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Where(where, args...).Count(&count).Error)
	return count
}

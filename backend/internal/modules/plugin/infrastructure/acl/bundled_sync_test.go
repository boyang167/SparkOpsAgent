package acl_test

import (
	"context"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/infrastructure/acl"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

const oldKubernetesMenuManifest = `id: kubernetes
version: "0.1.0"
display_name: Kubernetes
route_prefix: /api/v1/plugins/kubernetes
entitlement: plugin:kubernetes
menus:
  - title: Kubernetes
    path: /plugins/kubernetes
    component: plugins/kubernetes/page
  - title: 工作负载
    path: /plugins/kubernetes/workloads
    component: plugins/kubernetes/workloads/page
    parent: /plugins/kubernetes
  - title: OpenKruise
    path: /plugins/kubernetes/openkruise
    component: plugins/kubernetes/openkruise/page
    parent: /plugins/kubernetes
  - title: CloudTTY
    path: /plugins/kubernetes/cloudtty
    component: plugins/kubernetes/cloudtty/page
    parent: /plugins/kubernetes
apis:
  - method: GET
    path: /api/v1/plugins/kubernetes/ready
frontend:
  pages:
    - app/(platform)/plugins/kubernetes/page.tsx
`

const newKubernetesMenuManifest = `id: kubernetes
version: "0.1.0"
display_name: Kubernetes
route_prefix: /api/v1/plugins/kubernetes
entitlement: plugin:kubernetes
menus:
  - title: Kubernetes
    path: /plugins/kubernetes
    component: plugins/kubernetes/page
  - title: 工作负载
    path: /plugins/kubernetes/workloads
    component: plugins/kubernetes/workloads/page
    parent: /plugins/kubernetes
  - title: CloudTTY
    path: /plugins/kubernetes/cloudtty
    component: plugins/kubernetes/cloudtty/page
    parent: /plugins/kubernetes
  - title: OpenKruise
    path: /plugins/kubernetes/openkruise
    component: plugins/kubernetes/openkruise/page
    parent: /plugins/kubernetes
apis:
  - method: GET
    path: /api/v1/plugins/kubernetes/ready
frontend:
  pages:
    - app/(platform)/plugins/kubernetes/page.tsx
`

func TestSyncBundledPluginsUpdatesMenuSortWhenManifestChanges(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	sparkplugin.RegisterBundled("kubernetes", sparkplugin.BundledDescriptor{
		ManifestYAML: newKubernetesMenuManifest,
	})

	db := openBinderDB(t)
	wsID := kernel.NewWorkspaceID()
	now := time.Now().UTC()
	require.NoError(t, db.Exec(
		`INSERT INTO plugin.plugins (
			id, version, display_name, manifest_yaml, package_digest, installed_files,
			status, pr_url, created_by, created_at, updated_at
		) VALUES (?, ?, ?, ?, '', '[]', ?, '', ?, ?, ?)`,
		"kubernetes", "0.1.0", "Kubernetes", oldKubernetesMenuManifest, "deployed",
		"00000000-0000-0000-0000-000000000000", now, now,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO plugin.workspace_bindings (workspace_id, plugin_id, enabled) VALUES (?, ?, 1)`,
		wsID.String(), "kubernetes",
	).Error)

	oldSpec, err := manifest.Parse([]byte(oldKubernetesMenuManifest))
	require.NoError(t, err)
	require.NoError(t, acl.NewGovernanceBinder(db).Bind(context.Background(), wsID, oldSpec))

	var openKruiseSort int
	require.NoError(t, db.Table("governance.menus").Select("sort").
		Where("workspace_id = ? AND path = ?", wsID.String(), "/plugins/kubernetes/openkruise").
		Scan(&openKruiseSort).Error)
	require.Equal(t, 3, openKruiseSort)

	require.NoError(t, acl.SyncBundledPlugins(context.Background(), db))

	var storedManifest string
	require.NoError(t, db.Table("plugin.plugins").Select("manifest_yaml").
		Where("id = ?", "kubernetes").Scan(&storedManifest).Error)
	require.Equal(t, newKubernetesMenuManifest, storedManifest)

	require.NoError(t, db.Table("governance.menus").Select("sort").
		Where("workspace_id = ? AND path = ?", wsID.String(), "/plugins/kubernetes/openkruise").
		Scan(&openKruiseSort).Error)
	require.Equal(t, 4, openKruiseSort)
}

func TestSyncBundledPluginsRebindsWhenManifestAlreadyCurrent(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	sparkplugin.RegisterBundled("kubernetes", sparkplugin.BundledDescriptor{
		ManifestYAML: newKubernetesMenuManifest,
	})

	db := openBinderDB(t)
	wsID := kernel.NewWorkspaceID()
	now := time.Now().UTC()
	require.NoError(t, db.Exec(
		`INSERT INTO plugin.plugins (
			id, version, display_name, manifest_yaml, package_digest, installed_files,
			status, pr_url, created_by, created_at, updated_at
		) VALUES (?, ?, ?, ?, '', '[]', ?, '', ?, ?, ?)`,
		"kubernetes", "0.1.0", "Kubernetes", newKubernetesMenuManifest, "deployed",
		"00000000-0000-0000-0000-000000000000", now, now,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO plugin.workspace_bindings (workspace_id, plugin_id, enabled) VALUES (?, ?, 1)`,
		wsID.String(), "kubernetes",
	).Error)

	oldSpec, err := manifest.Parse([]byte(oldKubernetesMenuManifest))
	require.NoError(t, err)
	require.NoError(t, acl.NewGovernanceBinder(db).Bind(context.Background(), wsID, oldSpec))

	var openKruiseSort int
	require.NoError(t, db.Table("governance.menus").Select("sort").
		Where("workspace_id = ? AND path = ? AND deleted_at IS NULL", wsID.String(), "/plugins/kubernetes/openkruise").
		Scan(&openKruiseSort).Error)
	require.Equal(t, 3, openKruiseSort)

	require.NoError(t, acl.SyncBundledPlugins(context.Background(), db))

	require.NoError(t, db.Table("governance.menus").Select("sort").
		Where("workspace_id = ? AND path = ? AND deleted_at IS NULL", wsID.String(), "/plugins/kubernetes/openkruise").
		Scan(&openKruiseSort).Error)
	require.Equal(t, 4, openKruiseSort)

	var duplicateCount int64
	require.NoError(t, db.Table("governance.menus").
		Where("workspace_id = ? AND path = ? AND deleted_at IS NULL", wsID.String(), "/plugins/kubernetes/openkruise").
		Count(&duplicateCount).Error)
	require.Equal(t, int64(1), duplicateCount)
}

func TestSyncBundledPluginsSkipsCustomPluginRecord(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	sparkplugin.RegisterBundled("kubernetes", sparkplugin.BundledDescriptor{
		ManifestYAML: newKubernetesMenuManifest,
	})

	db := openBinderDB(t)
	now := time.Now().UTC()
	require.NoError(t, db.Exec(
		`INSERT INTO plugin.plugins (
			id, version, display_name, manifest_yaml, package_digest, installed_files,
			status, pr_url, created_by, created_at, updated_at
		) VALUES (?, ?, ?, ?, '', '[]', ?, '', ?, ?, ?)`,
		"kubernetes", "9.9.9", "Custom Kubernetes", oldKubernetesMenuManifest, "uploaded",
		kernel.NewMemberID().String(), now, now,
	).Error)

	require.NoError(t, acl.SyncBundledPlugins(context.Background(), db))

	var storedManifest string
	require.NoError(t, db.Table("plugin.plugins").Select("manifest_yaml").
		Where("id = ?", "kubernetes").Scan(&storedManifest).Error)
	require.Equal(t, oldKubernetesMenuManifest, storedManifest)
}

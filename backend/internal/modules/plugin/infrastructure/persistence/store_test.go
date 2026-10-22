package persistence_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/infrastructure/persistence"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openPluginStore(t *testing.T) (*gorm.DB, repository.PluginRepository) {
	t.Helper()
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		t.Skip("DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, persistence.AutoMigrate(db))
	return db, persistence.NewStore(db)
}

func openSQLitePluginStore(t *testing.T) repository.PluginRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS plugin").Error)
	require.NoError(t, persistence.AutoMigrate(db))
	return persistence.NewStore(db)
}

func TestAutoMigrateCreatesPluginSchemaTables(t *testing.T) {
	db, _ := openPluginStore(t)

	for _, table := range []string{"plugin.plugins", "plugin.workspace_bindings", "plugin.uninstall_receipts"} {
		var exists bool
		require.NoError(t, db.Raw("SELECT to_regclass(?) IS NOT NULL", table).Scan(&exists).Error)
		require.Truef(t, exists, "missing table %s", table)
	}
	assertColumn(t, db, "plugins", "version", "text", false, "")
	assertColumn(t, db, "plugins", "display_name", "text", false, "")
	assertColumn(t, db, "plugins", "manifest_yaml", "text", false, "")
	assertColumn(t, db, "plugins", "package_digest", "text", false, "")
	assertColumn(t, db, "plugins", "installed_files", "jsonb", false, "[]")
	assertColumn(t, db, "plugins", "status", "text", false, "")
	assertColumn(t, db, "plugins", "created_by", "uuid", false, "")
	assertColumn(t, db, "workspace_bindings", "workspace_id", "uuid", false, "")
	assertColumn(t, db, "workspace_bindings", "plugin_id", "text", false, "")
	assertColumn(t, db, "workspace_bindings", "enabled", "boolean", false, "false")

	var fkCount int64
	require.NoError(t, db.Raw(`
		SELECT count(*)
		FROM pg_constraint c
		JOIN pg_class rel ON rel.oid = c.conrelid
		JOIN pg_namespace ns ON ns.oid = rel.relnamespace
		WHERE ns.nspname = 'plugin'
		  AND rel.relname = 'workspace_bindings'
		  AND c.conname = 'fk_plugin_workspace_bindings_plugin'
		  AND c.contype = 'f'
	`).Scan(&fkCount).Error)
	require.Equal(t, int64(1), fkCount)
}

func TestAutoMigrateSeedsBundledExamplePlugin(t *testing.T) {
	repo := openSQLitePluginStore(t)
	found, err := repo.FindPlugin(context.Background(), "example")
	require.NoError(t, err)
	require.Equal(t, model.PluginStatusDeployed, found.Status)
	require.Equal(t, "example", found.ID)
}

func TestAutoMigrateSeedsBundledKubernetesPlugin(t *testing.T) {
	registerBundledKubernetesForTest(t)
	repo := openSQLitePluginStore(t)
	found, err := repo.FindPlugin(context.Background(), "kubernetes")
	require.NoError(t, err)
	require.Equal(t, model.PluginStatusDeployed, found.Status)
	require.Equal(t, "kubernetes", found.ID)
	require.Contains(t, found.InstalledFiles, "backend/internal/plugins/kubernetes/plugin.go")
	require.Contains(t, found.InstalledFiles, "backend/internal/plugins/kubernetes/plugin.yaml")
	require.Contains(t, found.InstalledFiles, "backend/internal/plugins/kubernetes/files/handler.go")
	require.Contains(t, found.InstalledFiles, "backend/internal/plugins/kubernetes/http/router.go")
	require.Contains(t, found.InstalledFiles, "backend/internal/plugins/kubernetes/resource/arthas-bin.tar")
	require.Contains(t, found.InstalledFiles, "fronted/apps/web/app/(platform)/plugins/kubernetes/page.tsx")
	require.Contains(t, found.InstalledFiles, "fronted/apps/web/app/(platform)/plugins/kubernetes/access-controls/page.tsx")
	require.Contains(t, found.InstalledFiles, "fronted/apps/web/src/modules/plugins/kubernetes/catalog.ts")

	spec, err := manifest.Parse([]byte(found.ManifestYAML))
	require.NoError(t, err)
	require.Equal(t, "kubernetes", spec.ID)
	require.Equal(t, "/api/v1/plugins/kubernetes", spec.RoutePrefix)
	require.Equal(t, "plugin:kubernetes", spec.Entitlement)
	require.Equal(t, []string{"backend/internal/plugins/kubernetes/resource/arthas-bin.tar"}, spec.Assets)
	require.Equal(t, []string{
		"app/(platform)/plugins/kubernetes/page.tsx",
		"app/(platform)/plugins/kubernetes/layout.tsx",
		"app/(platform)/plugins/kubernetes/clusters/page.tsx",
		"app/(platform)/plugins/kubernetes/nodes/page.tsx",
		"app/(platform)/plugins/kubernetes/workloads/page.tsx",
		"app/(platform)/plugins/kubernetes/namespaces/page.tsx",
		"app/(platform)/plugins/kubernetes/networks/page.tsx",
		"app/(platform)/plugins/kubernetes/gateway/page.tsx",
		"app/(platform)/plugins/kubernetes/configs/page.tsx",
		"app/(platform)/plugins/kubernetes/storages/page.tsx",
		"app/(platform)/plugins/kubernetes/access-controls/page.tsx",
		"app/(platform)/plugins/kubernetes/backup/page.tsx",
		"app/(platform)/plugins/kubernetes/records/page.tsx",
		"app/(platform)/plugins/kubernetes/ops-tools/page.tsx",
		"app/(platform)/plugins/kubernetes/diagnosis/page.tsx",
		"app/(platform)/plugins/kubernetes/chat/page.tsx",
		"app/(platform)/plugins/kubernetes/cloudtty/page.tsx",
		"app/(platform)/plugins/kubernetes/openkruise/page.tsx",
		"app/(platform)/plugins/kubernetes/kruise-rollouts/page.tsx",
		"app/(platform)/plugins/kubernetes/kubeblocks/page.tsx",
		"app/(platform)/plugins/kubernetes/kubevela/page.tsx",
		"app/(platform)/plugins/kubernetes/velaux/page.tsx",
	}, spec.Frontend.Pages)

	opsToolPaths := map[string]struct{}{
		"/plugins/kubernetes/diagnosis":       {},
		"/plugins/kubernetes/backup":          {},
		"/plugins/kubernetes/chat":            {},
		"/plugins/kubernetes/cloudtty":        {},
		"/plugins/kubernetes/openkruise":      {},
		"/plugins/kubernetes/kruise-rollouts": {},
		"/plugins/kubernetes/kubeblocks":      {},
		"/plugins/kubernetes/kubevela":        {},
		"/plugins/kubernetes/velaux":          {},
	}
	for _, path := range []string{
		"/plugins/kubernetes",
		"/plugins/kubernetes/clusters",
		"/plugins/kubernetes/nodes",
		"/plugins/kubernetes/workloads",
		"/plugins/kubernetes/namespaces",
		"/plugins/kubernetes/networks",
		"/plugins/kubernetes/gateway",
		"/plugins/kubernetes/configs",
		"/plugins/kubernetes/storages",
		"/plugins/kubernetes/access-controls",
		"/plugins/kubernetes/records",
		"/plugins/kubernetes/ops-tools",
		"/plugins/kubernetes/diagnosis",
		"/plugins/kubernetes/backup",
		"/plugins/kubernetes/chat",
		"/plugins/kubernetes/cloudtty",
		"/plugins/kubernetes/openkruise",
		"/plugins/kubernetes/kruise-rollouts",
		"/plugins/kubernetes/kubeblocks",
		"/plugins/kubernetes/kubevela",
		"/plugins/kubernetes/velaux",
	} {
		menu, ok := findMenu(spec.Menus, path)
		require.Truef(t, ok, "missing menu %s", path)
		switch {
		case path == "/plugins/kubernetes":
			require.Empty(t, menu.Parent)
		case path == "/plugins/kubernetes/ops-tools":
			require.Equal(t, "/plugins/kubernetes", menu.Parent)
		case func() bool { _, ok := opsToolPaths[path]; return ok }():
			require.Equal(t, "/plugins/kubernetes/ops-tools", menu.Parent)
		default:
			require.Equal(t, "/plugins/kubernetes", menu.Parent)
		}
	}

	requireAPI(t, spec.APIs, "GET", "/api/v1/plugins/kubernetes/ready", false)
	requireAPI(t, spec.APIs, "GET", "/api/v1/plugins/kubernetes/clusters", false)
	requireAPI(t, spec.APIs, "GET", "/api/v1/plugins/kubernetes/clusters/:clusterId/kubeconfig", false)
	requireAPI(t, spec.APIs, "GET", "/api/v1/plugins/kubernetes/clusters/:clusterId/users", false)
	requireAPI(t, spec.APIs, "POST", "/api/v1/plugins/kubernetes/clusters/:clusterId/users", false)
	requireAPI(t, spec.APIs, "PUT", "/api/v1/plugins/kubernetes/clusters/:clusterId/users/:memberId", false)
	requireAPI(t, spec.APIs, "DELETE", "/api/v1/plugins/kubernetes/clusters/:clusterId/users/:memberId", false)
	requireAPI(t, spec.APIs, "POST", "/api/v1/plugins/kubernetes/clusters/:clusterId/users/:memberId/kubeconfig", false)
	requireAPI(t, spec.APIs, "PUT", "/api/v1/plugins/kubernetes/clusters/:clusterId/settings", false)
	requireAPI(t, spec.APIs, "ANY", "/api/v1/plugins/kubernetes/clusters/:clusterId/proxy/*", false)
	requireAPI(t, spec.APIs, "POST", "/api/v1/plugins/kubernetes/clusters/:clusterId/metrics", true)
	requireAPI(t, spec.APIs, "GET", "/api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/logs", false)
	requireAPI(t, spec.APIs, "GET", "/api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/exec", false)
	requireAPI(t, spec.APIs, "GET", "/api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/files", false)
	requireAPI(t, spec.APIs, "GET", "/api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/files/download", false)
	requireAPI(t, spec.APIs, "POST", "/api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/files", false)
	requireAPI(t, spec.APIs, "DELETE", "/api/v1/plugins/kubernetes/clusters/:clusterId/namespaces/:namespace/pods/:pod/files", false)
	requireAPI(t, spec.APIs, "GET", "/api/v1/plugins/kubernetes/records", false)
	requireAPI(t, spec.APIs, "GET", "/api/v1/plugins/kubernetes/records/:recordId", false)
	requireAPI(t, spec.APIs, "DELETE", "/api/v1/plugins/kubernetes/records/:recordId", false)
	requireAPI(t, spec.APIs, "POST", "/api/v1/plugins/kubernetes/clusters/:clusterId/cloudtty", false)
	requireAPI(t, spec.APIs, "POST", "/api/v1/plugins/kubernetes/clusters/:clusterId/nodetty", false)
	requireAPI(t, spec.APIs, "POST", "/api/v1/plugins/kubernetes/clusters/:clusterId/diagnosis", false)
	requireAPI(t, spec.APIs, "GET", "/api/v1/plugins/kubernetes/diagnosis/:id/ws", false)
	requireAPI(t, spec.APIs, "DELETE", "/api/v1/plugins/kubernetes/diagnosis/:id", false)
	requireAPI(t, spec.APIs, "ANY", "/api/v1/plugins/kubernetes/clusters/:clusterId/velaux/*", false)
	requireAPI(t, spec.APIs, "POST", "/api/v1/plugins/kubernetes/clusters/:clusterId/chat/query", false)
}

func TestStoreCompletesUninstallWithDurableReceipt(t *testing.T) {
	repo := openSQLitePluginStore(t)
	ctx := context.Background()
	pluginID := fmt.Sprintf("receipt-%s", uuid.NewString())
	require.NoError(t, repo.SavePlugin(ctx, &model.PluginRecord{
		ID:            pluginID,
		Version:       "0.1.0",
		DisplayName:   "Receipt Plugin",
		ManifestYAML:  "id: receipt",
		PackageDigest: strings.Repeat("a", 64),
		Status:        model.PluginStatusUninstallMerged,
		CreatedBy:     kernel.NewMemberID(),
	}))
	deployedAt := time.Now().UTC().Truncate(time.Microsecond)

	require.NoError(t, repo.CompleteUninstall(ctx, pluginID, deployedAt))
	_, err := repo.FindPlugin(ctx, pluginID)
	require.ErrorIs(t, err, domain.ErrPluginNotFound)
	found, err := repo.HasUninstallReceipt(ctx, pluginID)
	require.NoError(t, err)
	require.True(t, found)

	unknownID := fmt.Sprintf("unknown-%s", uuid.NewString())
	require.ErrorIs(t, repo.CompleteUninstall(ctx, unknownID, deployedAt), domain.ErrPluginNotFound)
	found, err = repo.HasUninstallReceipt(ctx, unknownID)
	require.NoError(t, err)
	require.False(t, found)
}

func TestStoreSavePluginRejectsConflictingPackageDigest(t *testing.T) {
	repo := openSQLitePluginStore(t)
	ctx := context.Background()
	pluginID := fmt.Sprintf("conflict-%s", uuid.NewString())
	first := &model.PluginRecord{
		ID:            pluginID,
		Version:       "0.1.0",
		DisplayName:   "Conflict",
		ManifestYAML:  "id: conflict",
		PackageDigest: strings.Repeat("a", 64),
		Status:        model.PluginStatusUploaded,
		CreatedBy:     kernel.NewMemberID(),
	}
	require.NoError(t, repo.SavePlugin(ctx, first))
	second := *first
	second.PackageDigest = strings.Repeat("b", 64)
	require.ErrorIs(t, repo.SavePlugin(ctx, &second), domain.ErrPluginIDConflict)
	found, err := repo.FindPlugin(ctx, pluginID)
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("a", 64), found.PackageDigest)

	stale := *first
	stale.Status = model.PluginStatusPROpen
	stale.PRURL = "https://example.local/pr/1"
	require.NoError(t, repo.SavePlugin(ctx, &stale))
	found, err = repo.FindPlugin(ctx, pluginID)
	require.NoError(t, err)
	require.Equal(t, model.PluginStatusPROpen, found.Status)
	require.Equal(t, strings.Repeat("a", 64), found.PackageDigest)
}

func TestStorePersistsPluginsAndWorkspaceBindings(t *testing.T) {
	db, repo := openPluginStore(t)
	ctx := context.Background()
	pluginID := fmt.Sprintf("example-%s", uuid.NewString())
	wsID := kernel.NewWorkspaceID()
	createdBy := kernel.NewMemberID()
	enabledBy := kernel.NewMemberID()
	enabledAt := time.Now().UTC().Truncate(time.Microsecond)
	plugin := &model.PluginRecord{
		ID:            pluginID,
		Version:       "0.1.0",
		DisplayName:   "Example Plugin",
		ManifestYAML:  "id: example",
		PackageDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		InstalledFiles: []string{
			"backend/internal/plugins/example/plugin.go",
			"fronted/apps/web/app/(platform)/plugins/example/page.tsx",
		},
		Status:    model.PluginStatusUploaded,
		CreatedBy: createdBy,
		CreatedAt: enabledAt.Add(-time.Hour),
		UpdatedAt: enabledAt.Add(-time.Hour),
	}
	t.Cleanup(func() {
		_ = repo.DeleteBinding(ctx, wsID, pluginID)
		_ = repo.DeletePlugin(ctx, pluginID)
	})

	require.NoError(t, repo.SavePlugin(ctx, plugin))
	found, err := repo.FindPlugin(ctx, pluginID)
	require.NoError(t, err)
	require.Equal(t, pluginID, found.ID)
	require.Equal(t, "0.1.0", found.Version)
	require.Equal(t, "Example Plugin", found.DisplayName)
	require.Equal(t, plugin.PackageDigest, found.PackageDigest)
	require.Equal(t, plugin.InstalledFiles, found.InstalledFiles)
	require.Equal(t, model.PluginStatusUploaded, found.Status)
	require.Equal(t, createdBy, found.CreatedBy)

	require.NoError(t, found.MarkPROpen("https://example.test/pr/1"))
	require.NoError(t, repo.SavePlugin(ctx, found))
	reopened, err := repo.FindPlugin(ctx, pluginID)
	require.NoError(t, err)
	require.Equal(t, model.PluginStatusPROpen, reopened.Status)
	require.Equal(t, "https://example.test/pr/1", reopened.PRURL)

	require.NoError(t, reopened.MarkMerged())
	require.NoError(t, reopened.MarkDeployed())
	require.NoError(t, repo.SavePlugin(ctx, reopened))
	deployed, err := repo.FindPlugin(ctx, pluginID)
	require.NoError(t, err)
	require.True(t, deployed.CanEnable())

	binding := &model.WorkspaceBinding{
		WorkspaceID: wsID,
		PluginID:    pluginID,
		Enabled:     true,
		EnabledBy:   enabledBy,
		EnabledAt:   enabledAt,
	}
	require.NoError(t, repo.SaveBinding(ctx, binding))

	foundBinding, err := repo.FindBinding(ctx, wsID, pluginID)
	require.NoError(t, err)
	require.Equal(t, wsID, foundBinding.WorkspaceID)
	require.Equal(t, pluginID, foundBinding.PluginID)
	require.True(t, foundBinding.Enabled)
	require.Equal(t, enabledBy, foundBinding.EnabledBy)
	require.Equal(t, enabledAt, foundBinding.EnabledAt)

	bindingsByPlugin, err := repo.ListBindingsByPlugin(ctx, pluginID)
	require.NoError(t, err)
	require.Len(t, bindingsByPlugin, 1)
	require.Equal(t, wsID, bindingsByPlugin[0].WorkspaceID)

	enabledByWorkspace, err := repo.ListEnabledByWorkspace(ctx, wsID)
	require.NoError(t, err)
	require.Contains(t, pluginIDs(enabledByWorkspace), pluginID)

	require.NoError(t, repo.DeletePlugin(ctx, pluginID))
	_, err = repo.FindBinding(ctx, wsID, pluginID)
	require.ErrorIs(t, err, domain.ErrPluginBindingNotFound)

	_, err = repo.FindPlugin(ctx, pluginID)
	require.ErrorIs(t, err, domain.ErrPluginNotFound)

	var remaining int64
	require.NoError(t, db.Table("plugin.plugins").Where("id = ?", pluginID).Count(&remaining).Error)
	require.Zero(t, remaining)
}

func assertColumn(t *testing.T, db *gorm.DB, table, column, dataType string, nullable bool, defaultValue string) {
	t.Helper()
	var row struct {
		DataType      string
		IsNullable    string
		ColumnDefault *string
	}
	require.NoError(t, db.Raw(`
		SELECT data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = 'plugin'
		  AND table_name = ?
		  AND column_name = ?
	`, table, column).Scan(&row).Error)
	require.Equal(t, dataType, row.DataType)
	if nullable {
		require.Equal(t, "YES", row.IsNullable)
	} else {
		require.Equal(t, "NO", row.IsNullable)
	}
	if defaultValue != "" {
		require.NotNil(t, row.ColumnDefault)
		require.Contains(t, *row.ColumnDefault, defaultValue)
	}
}

func pluginIDs(bindings []*model.WorkspaceBinding) []string {
	out := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, binding.PluginID)
	}
	return out
}

func findMenu(menus []manifest.MenuSpec, path string) (manifest.MenuSpec, bool) {
	for _, menu := range menus {
		if menu.Path == path {
			return menu, true
		}
	}
	return manifest.MenuSpec{}, false
}

func requireAPI(t *testing.T, apis []manifest.APISpec, method, path string, readOnly bool) {
	t.Helper()
	found := slices.ContainsFunc(apis, func(api manifest.APISpec) bool {
		return api.Method == method && api.Path == path && api.ReadOnly == readOnly
	})
	require.Truef(t, found, "missing api %s %s read_only=%v", method, path, readOnly)
}

func registerBundledKubernetesForTest(t *testing.T) {
	t.Helper()
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "plugins", "kubernetes", "plugin.yaml"))
	require.NoError(t, err)
	sparkplugin.RegisterBundled("kubernetes", sparkplugin.BundledDescriptor{
		ManifestYAML: string(raw),
		InstalledFiles: []string{
			"backend/internal/plugins/kubernetes/plugin.go",
			"backend/internal/plugins/kubernetes/plugin.yaml",
			"backend/internal/plugins/kubernetes/files/handler.go",
			"backend/internal/plugins/kubernetes/http/router.go",
			"backend/internal/plugins/kubernetes/resource/arthas-bin.tar",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/page.tsx",
			"fronted/apps/web/app/(platform)/plugins/kubernetes/access-controls/page.tsx",
			"fronted/apps/web/src/modules/plugins/kubernetes/catalog.ts",
		},
	})
}

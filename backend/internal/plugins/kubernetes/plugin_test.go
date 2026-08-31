package kubernetes

import (
	"archive/tar"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPluginReadyRouteRegistersWithContext(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	sparkplugin.Register(&plugin{})
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	memberID := kernel.NewMemberID()
	wsID := kernel.NewWorkspaceID()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})

	sparkplugin.InstallAllWithContext(engine, sparkplugin.PluginContext{
		Logger:  zap.NewNop(),
		Members: pluginMembers{wsID: wsID, memberID: memberID},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/kubernetes/ready", nil)
	req.Header.Set("X-Workspace-Id", wsID.String())
	engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"ok":true}`, w.Body.String())
}

func TestPluginSupportsLegacyAndContextEntrypointsOnSeparateEngines(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &plugin{}
	memberID := kernel.NewMemberID()
	wsID := kernel.NewWorkspaceID()

	legacyEngine := readyEngine(memberID)
	p.Register(legacyEngine)
	requireReadyStatus(t, legacyEngine, wsID, http.StatusServiceUnavailable)

	contextEngine := readyEngine(memberID)
	p.RegisterWithContext(contextEngine, sparkplugin.PluginContext{
		Logger:  zap.NewNop(),
		Members: pluginMembers{wsID: wsID, memberID: memberID},
	})
	requireReady(t, contextEngine, wsID)
}

func TestPluginImplementsContextPlugin(t *testing.T) {
	var _ sparkplugin.ContextPlugin = (*plugin)(nil)
}

func TestRegisterBundledDescriptor(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)

	registerBundledDescriptor()
	descriptor, ok := sparkplugin.Bundled("kubernetes")
	require.True(t, ok)
	require.Equal(t, ManifestYAML, descriptor.ManifestYAML)
	require.ElementsMatch(t, []string{
		"backend/internal/plugins/kubernetes/plugin.go",
		"backend/internal/plugins/kubernetes/plugin.yaml",
		"backend/internal/plugins/kubernetes/chat/query.go",
		"backend/internal/plugins/kubernetes/diagnosis/arthas.go",
		"backend/internal/plugins/kubernetes/files/handler.go",
		"backend/internal/plugins/kubernetes/http/clusters.go",
		"backend/internal/plugins/kubernetes/http/cluster_users.go",
		"backend/internal/plugins/kubernetes/http/kubeconfig.go",
		"backend/internal/plugins/kubernetes/http/metrics.go",
		"backend/internal/plugins/kubernetes/http/router.go",
		"backend/internal/plugins/kubernetes/http/settings.go",
		"backend/internal/plugins/kubernetes/http/settings_store.go",
		"backend/internal/plugins/kubernetes/http/write_guard.go",
		"backend/internal/plugins/kubernetes/metrics/query.go",
		"backend/internal/plugins/kubernetes/proxy/aggregate.go",
		"backend/internal/plugins/kubernetes/proxy/auth.go",
		"backend/internal/plugins/kubernetes/proxy/discovery_cache.go",
		"backend/internal/plugins/kubernetes/proxy/handler.go",
		"backend/internal/plugins/kubernetes/proxy/path.go",
		"backend/internal/plugins/kubernetes/proxy/redaction.go",
		"backend/internal/plugins/kubernetes/proxy/rewrite.go",
		"backend/internal/plugins/kubernetes/proxy/transport.go",
		"backend/internal/plugins/kubernetes/rbac/apply.go",
		"backend/internal/plugins/kubernetes/rbac/names.go",
		"backend/internal/plugins/kubernetes/records/handler.go",
		"backend/internal/plugins/kubernetes/resource/arthas-bin.tar",
		"backend/internal/plugins/kubernetes/resource/assets.go",
		"backend/internal/plugins/kubernetes/schema/migrate.go",
		"backend/internal/plugins/kubernetes/schema/models.go",
		"backend/internal/plugins/kubernetes/stream/exec.go",
		"backend/internal/plugins/kubernetes/stream/logs.go",
		"backend/internal/plugins/kubernetes/tty/cloudtty.go",
		"backend/internal/plugins/kubernetes/tty/nodetty.go",
		"backend/internal/plugins/kubernetes/velaux/proxy.go",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/layout.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/clusters/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/nodes/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/workloads/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/openkruise/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/kruise-rollouts/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/kubeblocks/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/kubevela/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/namespaces/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/networks/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/gateway/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/configs/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/storages/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/access-controls/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/backup/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/records/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/diagnosis/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/chat/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/cloudtty/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/velaux/page.tsx",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/create/page.tsx",
		"fronted/apps/web/app/api/bff/v1/plugins/kubernetes/[...path]/route.ts",
		"fronted/apps/web/src/modules/plugins/kubernetes/ApiErrorBanner.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/ClusterPicker.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/ClusterCreateForm.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/cluster-settings-form.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/cluster-status.ts",
		"fronted/apps/web/src/modules/plugins/kubernetes/cluster-types.ts",
		"fronted/apps/web/src/modules/plugins/kubernetes/KubernetesConsole.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/OperationsPanel.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/ResourceMenuPage.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/ResourceTable.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/YamlEditor.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/api.ts",
		"fronted/apps/web/src/modules/plugins/kubernetes/catalog.ts",
		"fronted/apps/web/src/modules/plugins/kubernetes/page-server.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/operations.ts",
		"fronted/apps/web/src/modules/plugins/kubernetes/resource-url.ts",
		"fronted/apps/web/src/modules/plugins/kubernetes/sse.ts",
		"fronted/apps/web/src/modules/plugins/kubernetes/velaux-frame.ts",
		"fronted/apps/web/src/modules/plugins/kubernetes/yaml-document.ts",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/ChatPage.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/CloudTTYPage.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClusterAssociateDialog.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClusterAuthorizeDialog.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClusterAuthorizeRoles.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/cluster-authorize-roles.ts",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClusterAuthorizeUsers.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClustersPanel.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/CreateDeploymentPage.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/DiagnosisPage.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/FormParts.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/OverviewPanel.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/RecordsPage.tsx",
		"fronted/apps/web/src/modules/plugins/kubernetes/specialized/VelauxPage.tsx",
	}, descriptor.InstalledFiles)
}

func TestArthasTarContainsBootJar(t *testing.T) {
	f, err := os.Open(filepath.Join("resource", "arthas-bin.tar"))
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	requireArthasTarContainsBootJar(t, f)
}

func requireArthasTarContainsBootJar(t *testing.T, r io.Reader) {
	t.Helper()

	tr := tar.NewReader(r)
	foundBootJar := false
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		requireSafeTarHeader(t, header)
		if header.Name == "arthas-bin/arthas-boot.jar" {
			require.Positive(t, header.Size)
			foundBootJar = true
		}
	}
	require.True(t, foundBootJar, "arthas-bin/arthas-boot.jar not found")
}

func requireSafeTarHeader(t *testing.T, header *tar.Header) {
	t.Helper()

	require.NotContains(t, header.Name, `\`)
	require.False(t, strings.HasPrefix(header.Name, "/"), header.Name)
	cleanName := strings.TrimSuffix(header.Name, "/")
	require.NotEmpty(t, cleanName)
	require.Equal(t, path.Clean(cleanName), cleanName)
	for _, segment := range strings.Split(cleanName, "/") {
		require.NotEmpty(t, segment, header.Name)
		require.NotEqual(t, ".", segment, header.Name)
		require.NotEqual(t, "..", segment, header.Name)
	}
	require.NotEqual(t, tar.TypeSymlink, header.Typeflag, header.Name)
	require.NotEqual(t, tar.TypeLink, header.Typeflag, header.Name)
}

func readyEngine(memberID kernel.MemberID) *gin.Engine {
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	return engine
}

func requireReady(t *testing.T, engine *gin.Engine, wsID kernel.WorkspaceID) {
	t.Helper()
	requireReadyStatus(t, engine, wsID, http.StatusOK)
}

func requireReadyStatus(t *testing.T, engine *gin.Engine, wsID kernel.WorkspaceID, status int) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/kubernetes/ready", nil)
	req.Header.Set("X-Workspace-Id", wsID.String())
	engine.ServeHTTP(w, req)
	require.Equal(t, status, w.Code)
	if status == http.StatusOK {
		require.JSONEq(t, `{"ok":true}`, w.Body.String())
	}
}

type pluginMembers struct {
	wsID     kernel.WorkspaceID
	memberID kernel.MemberID
}

func (m pluginMembers) IsWorkspaceMember(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return true, nil
}

package kubernetes

import (
	"archive/tar"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		"backend/internal/plugins/kubernetes/files/handler.go",
		"backend/internal/plugins/kubernetes/http/clusters.go",
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
		"backend/internal/plugins/kubernetes/records/handler.go",
		"backend/internal/plugins/kubernetes/resource/arthas-bin.tar",
		"backend/internal/plugins/kubernetes/stream/exec.go",
		"backend/internal/plugins/kubernetes/stream/logs.go",
		"fronted/apps/web/app/(platform)/plugins/kubernetes/page.tsx",
	}, descriptor.InstalledFiles)
}

func TestArthasTarContainsPlaceholderMember(t *testing.T) {
	f, err := os.Open(filepath.Join("resource", "arthas-bin.tar"))
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	header, err := tar.NewReader(f).Next()
	require.NoError(t, err)
	require.Equal(t, "README.placeholder", header.Name)
	require.Zero(t, header.Size)
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

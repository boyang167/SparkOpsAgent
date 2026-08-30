package kubernetes

import (
	"archive/tar"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPluginReadyRouteRegistersWithContext(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	sparkplugin.Register(&plugin{})
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	sparkplugin.InstallAllWithContext(engine, sparkplugin.PluginContext{Logger: zap.NewNop()})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/kubernetes/ready", nil)
	engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"ok":true}`, w.Body.String())
}

func TestPluginSupportsLegacyAndContextEntrypointsOnSeparateEngines(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := &plugin{}

	legacyEngine := gin.New()
	p.Register(legacyEngine)
	requireReady(t, legacyEngine)

	contextEngine := gin.New()
	p.RegisterWithContext(contextEngine, sparkplugin.PluginContext{Logger: zap.NewNop()})
	requireReady(t, contextEngine)
}

func TestPluginImplementsContextPlugin(t *testing.T) {
	var _ sparkplugin.ContextPlugin = (*plugin)(nil)
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

func requireReady(t *testing.T, engine *gin.Engine) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/kubernetes/ready", nil)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"ok":true}`, w.Body.String())
}

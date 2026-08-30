package plugin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
)

type pingPlugin struct{}

func (pingPlugin) Register(engine *gin.Engine) {
	engine.GET("/api/v1/plugins/example/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
}

type panicPlugin struct{}

func (panicPlugin) Register(*gin.Engine) { panic("boom") }

func TestRegisterIgnoresNilAndSnapshots(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	sparkplugin.Register(nil)
	sparkplugin.Register(pingPlugin{})
	got := sparkplugin.Registered()
	require.Len(t, got, 1)
	sparkplugin.Register(pingPlugin{})
	require.Len(t, sparkplugin.Registered(), 2)
	require.Len(t, got, 1)
}

func TestInstallAllRecoversPanicAndKeepsLaterPlugins(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	sparkplugin.Register(panicPlugin{})
	sparkplugin.Register(pingPlugin{})
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sparkplugin.InstallAll(engine, zap.NewNop())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/example/ping", nil)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRegisterBundledRejectsEmptyInputsAndReturnsUnknown(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)

	sparkplugin.RegisterBundled("", sparkplugin.BundledDescriptor{
		ManifestYAML:   "id: ignored",
		InstalledFiles: []string{"backend/internal/plugins/ignored/plugin.go"},
	})
	sparkplugin.RegisterBundled("empty-manifest", sparkplugin.BundledDescriptor{
		InstalledFiles: []string{"backend/internal/plugins/ignored/plugin.go"},
	})

	_, ok := sparkplugin.Bundled("")
	require.False(t, ok)
	_, ok = sparkplugin.Bundled("empty-manifest")
	require.False(t, ok)
	_, ok = sparkplugin.Bundled("missing")
	require.False(t, ok)
}

func TestRegisterBundledSnapshotsAndReturnsInstalledFileCopies(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)

	files := []string{"backend/internal/plugins/kubernetes/plugin.go"}
	sparkplugin.RegisterBundled("kubernetes", sparkplugin.BundledDescriptor{
		ManifestYAML:   "id: kubernetes",
		InstalledFiles: files,
	})
	files[0] = "mutated"

	first, ok := sparkplugin.Bundled("kubernetes")
	require.True(t, ok)
	require.Equal(t, "id: kubernetes", first.ManifestYAML)
	require.Equal(t, []string{"backend/internal/plugins/kubernetes/plugin.go"}, first.InstalledFiles)

	first.InstalledFiles[0] = "mutated-again"
	second, ok := sparkplugin.Bundled("kubernetes")
	require.True(t, ok)
	require.Equal(t, []string{"backend/internal/plugins/kubernetes/plugin.go"}, second.InstalledFiles)
}

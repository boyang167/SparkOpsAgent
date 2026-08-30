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

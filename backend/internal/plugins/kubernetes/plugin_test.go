package kubernetes

import (
	"net/http"
	"net/http/httptest"
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

func TestPluginRegistrationIsIdempotentAcrossLegacyAndContextEntrypoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	p := &plugin{}

	require.NotPanics(t, func() {
		p.RegisterWithContext(engine, sparkplugin.PluginContext{Logger: zap.NewNop()})
		p.Register(engine)
	})
}

func TestPluginImplementsContextPlugin(t *testing.T) {
	var _ sparkplugin.ContextPlugin = (*plugin)(nil)
}

package plugin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"

	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
)

type legacyOnly struct{}

func (legacyOnly) Register(engine *gin.Engine) {
	engine.GET("/api/v1/plugins/legacy/ping", func(c *gin.Context) { c.String(http.StatusOK, "legacy") })
}

type withCtx struct{ sawDB bool }

func (p *withCtx) Register(*gin.Engine) {}

func (p *withCtx) RegisterWithContext(engine *gin.Engine, ctx sparkplugin.PluginContext) {
	p.sawDB = ctx.DB != nil
	engine.GET("/api/v1/plugins/ctx/ping", func(c *gin.Context) { c.String(http.StatusOK, "ctx") })
}

func TestInstallAllWithContextPrefersRegisterWithContext(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	p := &withCtx{}
	sparkplugin.Register(legacyOnly{})
	sparkplugin.Register(p)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sparkplugin.InstallAllWithContext(engine, sparkplugin.PluginContext{Logger: zap.NewNop(), DB: &gorm.DB{}})
	require.True(t, p.sawDB)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/legacy/ping", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "legacy", w.Body.String())

	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/ctx/ping", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "ctx", w.Body.String())
}

func TestInstallAllDelegatesToInstallAllWithContext(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	sparkplugin.Register(legacyOnly{})
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sparkplugin.InstallAll(engine, zap.NewNop())

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/legacy/ping", nil))
	require.Equal(t, http.StatusOK, w.Code)
}

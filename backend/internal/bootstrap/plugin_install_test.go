package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
)

type mwProbePlugin struct{}

func (mwProbePlugin) Register(engine *gin.Engine) {
	engine.GET("/api/v1/plugins/probe", func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString("mw"))
	})
}

func TestPluginRoutesSeeAuthMiddlewareWhenInstalledAfter(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	sparkplugin.Register(mwProbePlugin{})
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("mw", "auth")
		c.Next()
	})
	sparkplugin.InstallAll(engine, zap.NewNop())

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/probe", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "auth", w.Body.String())

	engineInstalledBeforeMiddleware := gin.New()
	sparkplugin.InstallAll(engineInstalledBeforeMiddleware, zap.NewNop())
	engineInstalledBeforeMiddleware.Use(func(c *gin.Context) {
		c.Set("mw", "auth")
		c.Next()
	})

	w = httptest.NewRecorder()
	engineInstalledBeforeMiddleware.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/probe", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, w.Body.String())
}

func TestRunInstallsPluginsAfterAuthMiddlewareBeforeRouteCollect(t *testing.T) {
	source, err := os.ReadFile("app.go")
	require.NoError(t, err)
	app := string(source)

	require.Contains(t, app, `_ "github.com/sparkops/sparkops-agent/backend/internal/plugins"`)
	require.Contains(t, app, `sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"`)

	entitlementsMiddleware := strings.Index(app, "c.Engine.Use(entitlements.Middleware(c.DB))")
	pluginAuth := strings.Index(app, "c.Engine.Use(authn.RequireMemberForPluginAPIs())")
	installAll := strings.Index(app, "sparkplugin.InstallAllWithContext(c.Engine, pluginContext)")
	routeCollect := strings.Index(app, "c.RouteRegistry = routes.Collect(c.Engine)")

	require.NotEqual(t, -1, entitlementsMiddleware)
	require.NotEqual(t, -1, pluginAuth)
	require.NotEqual(t, -1, installAll)
	require.NotEqual(t, -1, routeCollect)
	require.Less(t, entitlementsMiddleware, pluginAuth)
	require.Less(t, pluginAuth, installAll)
	require.Less(t, installAll, routeCollect)
	require.Contains(t, app, "sparkplugin.NewFactory(adminacl.NewClusterCredentialSource(adminRepo))")
}

package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
)

const routePrefix = "/api/v1/plugins/kubernetes"

func RegisterRoutes(engine *gin.Engine, contexts ...sparkplugin.PluginContext) {
	if engine == nil {
		return
	}
	var pctx sparkplugin.PluginContext
	if len(contexts) > 0 {
		pctx = contexts[0]
	}
	handler := newClusterHandler(pctx)
	group := engine.Group(routePrefix)
	group.Use(httpx.RequireWorkspace(), requireWorkspaceMember(pctx.Members))
	group.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	group.GET("/clusters", handler.listClusters)
	group.PUT("/clusters/:clusterId/settings", handler.putClusterSettings)
}

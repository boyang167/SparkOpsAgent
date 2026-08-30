package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	k8sfiles "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/files"
	k8sproxy "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/proxy"
	k8srecords "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/records"
	k8sstream "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/stream"
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
	metricsHandler := newMetricsHandler(pctx)
	filesHandler := k8sfiles.NewHandler(pctx)
	recordsHandler := k8srecords.NewHandler(pctx)
	streamHandler := k8sstream.NewHandler(pctx)
	group := engine.Group(routePrefix)
	group.Use(httpx.RequireWorkspace(), requireWorkspaceMember(pctx.Members))
	group.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	group.GET("/clusters", handler.listClusters)
	group.PUT("/clusters/:clusterId/settings", handler.putClusterSettings)
	group.POST("/clusters/:clusterId/metrics", metricsHandler.postMetrics)
	group.Any("/clusters/:clusterId/proxy/*path", k8sproxy.NewHandler(pctx).ServeGIN)
	group.GET("/clusters/:clusterId/namespaces/:namespace/pods/:pod/files", filesHandler.List)
	group.GET("/clusters/:clusterId/namespaces/:namespace/pods/:pod/files/download", filesHandler.Download)
	group.POST("/clusters/:clusterId/namespaces/:namespace/pods/:pod/files", filesHandler.Upload)
	group.DELETE("/clusters/:clusterId/namespaces/:namespace/pods/:pod/files", filesHandler.Delete)
	group.GET("/clusters/:clusterId/namespaces/:namespace/pods/:pod/logs", streamHandler.Logs)
	group.GET("/clusters/:clusterId/namespaces/:namespace/pods/:pod/exec", streamHandler.Exec)
	group.GET("/records", recordsHandler.List)
	group.GET("/records/:recordId", recordsHandler.Get)
	group.DELETE("/records/:recordId", recordsHandler.Delete)
}

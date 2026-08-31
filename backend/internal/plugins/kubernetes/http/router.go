package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	k8schat "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/chat"
	k8sdiagnosis "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/diagnosis"
	k8sfiles "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/files"
	k8sproxy "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/proxy"
	k8srecords "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/records"
	k8sstream "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/stream"
	k8sterminal "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/terminal"
	k8stty "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/tty"
	k8svelaux "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/velaux"
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
	terminalHandler := k8sterminal.NewHandler(pctx, k8sterminal.NewTicketStore(nil))
	ttyHandler := k8stty.NewHandler(pctx)
	diagnosisHandler := k8sdiagnosis.NewHandler(pctx)
	velauxHandler := k8svelaux.NewHandler(pctx)
	chatHandler := k8schat.NewHandler(pctx)
	group := engine.Group(routePrefix)
	group.Use(httpx.RequireWorkspace(), requireWorkspaceMember(pctx.Members))
	group.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	group.GET("/clusters", handler.listClusters)
	group.GET("/clusters/:clusterId/kubeconfig", handler.getKubeconfig)
	group.GET("/clusters/:clusterId/users", handler.listClusterUsers)
	group.POST("/clusters/:clusterId/users", handler.createClusterUsers)
	group.PUT("/clusters/:clusterId/users/:memberId", handler.updateClusterUser)
	group.DELETE("/clusters/:clusterId/users/:memberId", handler.deleteClusterUser)
	group.POST("/clusters/:clusterId/users/:memberId/kubeconfig", handler.issueClusterUserKubeconfig)
	group.PUT("/clusters/:clusterId/settings", handler.putClusterSettings)
	group.POST("/clusters/:clusterId/metrics", metricsHandler.postMetrics)
	group.Any("/clusters/:clusterId/proxy/*path", k8sproxy.NewHandler(pctx).ServeGIN)
	group.POST("/clusters/:clusterId/diagnosis", diagnosisHandler.Start)
	group.Any("/clusters/:clusterId/velaux/*path", velauxHandler.ServeGIN)
	group.POST("/clusters/:clusterId/chat/query", chatHandler.Query)
	group.GET("/clusters/:clusterId/namespaces/:namespace/pods/:pod/files", filesHandler.List)
	group.GET("/clusters/:clusterId/namespaces/:namespace/pods/:pod/files/download", filesHandler.Download)
	group.POST("/clusters/:clusterId/namespaces/:namespace/pods/:pod/files", filesHandler.Upload)
	group.DELETE("/clusters/:clusterId/namespaces/:namespace/pods/:pod/files", filesHandler.Delete)
	group.GET("/clusters/:clusterId/namespaces/:namespace/pods/:pod/logs", streamHandler.Logs)
	group.GET("/clusters/:clusterId/namespaces/:namespace/pods/:pod/exec", streamHandler.Exec)
	group.POST("/clusters/:clusterId/namespaces/:namespace/pods/:pod/terminal-ticket", terminalHandler.CreateTicket)
	group.GET("/records", recordsHandler.List)
	group.GET("/records/:recordId", recordsHandler.Get)
	group.DELETE("/records/:recordId", recordsHandler.Delete)
	group.GET("/diagnosis/:id/ws", diagnosisHandler.Stream)
	group.DELETE("/diagnosis/:id", diagnosisHandler.Delete)
	group.POST("/clusters/:clusterId/cloudtty", ttyHandler.CloudTTY)
	group.POST("/clusters/:clusterId/nodetty", ttyHandler.NodeTTY)
	engine.GET(k8sterminal.WebsocketPath, terminalHandler.ServeWS)
}

package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/api"
	"github.com/gin-gonic/gin"
)

type WsApiRouter struct{}

func (t *WsApiRouter) InitWsRouter(Router *gin.RouterGroup) {
	WsRouter := Router.Group("kubernetes/pods")
	wsApi := api.ApiGroupApp.WsApi
	podsApi := api.ApiGroupApp.PodsApi
	WsRouter.GET("/terminal", wsApi.Terminal)           // 终端
	WsRouter.GET("/arthasWs", wsApi.ArthasTerminal)     // Arthas终端
	WsRouter.GET("/logs", wsApi.ContainerLog)           // 终端日志
	WsRouter.GET("/downloadFile", podsApi.DownloadFile) // 文件下载
}

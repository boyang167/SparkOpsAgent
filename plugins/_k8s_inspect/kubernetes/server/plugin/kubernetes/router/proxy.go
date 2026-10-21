package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/api"
	"github.com/gin-gonic/gin"
)

type ProxyApiRouter struct{}

func (r *ProxyApiRouter) InitProxyRouter(Router *gin.RouterGroup) {
	ProxyRouter := Router.Group("kubernetes/proxy").Use(middleware.OperationRecord())
	ProxyApi := api.ApiGroupApp.ProxyApi
	ProxyRouter.Any("/:cluster_id/*path", ProxyApi.K8sAPIProxy) // 资源代理
}

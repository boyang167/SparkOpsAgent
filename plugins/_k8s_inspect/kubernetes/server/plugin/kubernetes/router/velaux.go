package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/api"
	"github.com/gin-gonic/gin"
)

type VelauxApiRouter struct{}

func (v *VelauxApiRouter) InitVelauxRouter(Router *gin.RouterGroup) {
	velauxProxyRouter := Router.Group("kubernetes/velaux").Use(middleware.OperationRecord())
	velauxApi := api.ApiGroupApp.VelauxApi
	velauxProxyRouter.Any("/:cluster_id/*path", velauxApi.VelauxAPIProxy) // 资源代理
}

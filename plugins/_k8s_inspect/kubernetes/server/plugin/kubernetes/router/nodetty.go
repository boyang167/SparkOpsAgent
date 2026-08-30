package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/api"
	"github.com/gin-gonic/gin"
)

type NodeTTYApiRouter struct{}

func (n *NodeTTYApiRouter) InitNodeTTYRouter(Router *gin.RouterGroup) {
	nodettyRouter := Router.Group("kubernetes/nodetty").Use(middleware.OperationRecord())
	nodettyApi := api.ApiGroupApp.NodeTTYApi
	nodettyRouter.POST("/get", nodettyApi.NodeTTYGet) // NodeTTY
}

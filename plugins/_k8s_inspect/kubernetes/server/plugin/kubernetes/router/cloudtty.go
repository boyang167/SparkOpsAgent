package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/api"
	"github.com/gin-gonic/gin"
)

type CloudTTYApiRouter struct{}

func (v *CloudTTYApiRouter) InitCloudTTYRouter(Router *gin.RouterGroup) {
	cloudTTYRouter := Router.Group("kubernetes/cloudtty").Use(middleware.OperationRecord())
	cloudTTYApi := api.ApiGroupApp.CloudTTYApi
	cloudTTYRouter.POST("/get", cloudTTYApi.CloudTTYGet) // CloudTTY
}

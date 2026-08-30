package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/api"
	"github.com/gin-gonic/gin"
)

type PodsRouter struct{}

func (p *PodsRouter) InitPodsRouter(Router *gin.RouterGroup) {
	podsRouter := Router.Group("kubernetes/pods").Use(middleware.OperationRecord())
	podsApi := api.ApiGroupApp.PodsApi
	{
		podsRouter.POST("uploadFile", podsApi.UploadFile) // 上传文件
		podsRouter.POST("deleteFile", podsApi.DeleteFile) // 删除文件
		podsRouter.POST("listFiles", podsApi.ListFiles)   // 文件获取列表
	}
}

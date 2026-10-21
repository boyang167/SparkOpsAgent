package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/api"
	"github.com/gin-gonic/gin"
)

type RecordsRouter struct{}

func (r *RecordsRouter) InitRecordsRouter(Router *gin.RouterGroup) {
	recordsRouter := Router.Group("kubernetes/records").Use(middleware.OperationRecord())
	recordsApi := api.ApiGroupApp.RecordsApi
	{
		recordsRouter.POST("getById", recordsApi.GetRecordById)           // 单个获取
		recordsRouter.DELETE("delete", recordsApi.DeleteRecord)           // 单个删除
		recordsRouter.DELETE("deleteByIds", recordsApi.DeleteRecordByIds) // 批量删除
		recordsRouter.POST("listRecord", recordsApi.ListRecord)           // 审计列表

	}
}

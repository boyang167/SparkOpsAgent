package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/api"
	"github.com/gin-gonic/gin"
)

type DiagnosisRouter struct{}

func (d *DiagnosisRouter) InitDiagnosisRouter(Router *gin.RouterGroup) {
	diagnosisRouter := Router.Group("kubernetes/diagnosis").Use(middleware.OperationRecord())
	diagnosisApi := api.ApiGroupApp.DiagnosisApi
	{
		diagnosisRouter.POST("jps", diagnosisApi.DiagnosisJps)               // 进程
		diagnosisRouter.POST("install", diagnosisApi.DiagnosisInstall)       // 安装
		diagnosisRouter.DELETE("uninstall", diagnosisApi.DiagnosisUninstall) // 卸载
		diagnosisRouter.POST("getByPod", diagnosisApi.DiagnosisByPod)        // 获取
		diagnosisRouter.POST("proxy", diagnosisApi.DiagnosisProxy)           // 代理

	}
}

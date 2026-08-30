package router

import (
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/api"
	"github.com/gin-gonic/gin"
)

type MetricRouter struct{}

func (u *MetricRouter) InitMetricRouter(Router *gin.RouterGroup) {
	metricRouter := Router.Group("kubernetes/metrics").Use(middleware.OperationRecord())
	metricsApi := api.ApiGroupApp.MetricsApi
	{
		metricRouter.POST("get", metricsApi.MetricsGet) // 监控数据获取
	}
}

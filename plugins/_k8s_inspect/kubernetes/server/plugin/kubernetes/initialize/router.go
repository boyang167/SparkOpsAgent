package initialize

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/router"
	"github.com/gin-gonic/gin"
)

func Router(engine *gin.Engine) {
	public := engine.Group(global.GVA_CONFIG.System.RouterPrefix).Group("")
	public.Use()
	private := engine.Group(global.GVA_CONFIG.System.RouterPrefix).Group("")
	private.Use(middleware.JWTAuth()).Use(middleware.CasbinHandler())
	router.RouterGroupApp.InitClusterRouter(private)
	router.RouterGroupApp.InitProxyRouter(private)
	router.RouterGroupApp.InitMetricRouter(private)
	router.RouterGroupApp.InitPodsRouter(private)
	router.RouterGroupApp.InitRecordsRouter(private)
	router.RouterGroupApp.InitVelauxRouter(private)
	router.RouterGroupApp.InitCloudTTYRouter(private)
	router.RouterGroupApp.InitNodeTTYRouter(private)
	router.RouterGroupApp.InitDiagnosisRouter(private)
	router.RouterGroupApp.InitChatRouter(private)
}

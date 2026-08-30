package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const routePrefix = "/api/v1/plugins/kubernetes"

func RegisterRoutes(engine *gin.Engine) {
	if engine == nil || hasRoute(engine, http.MethodGet, routePrefix+"/ready") {
		return
	}
	group := engine.Group(routePrefix)
	group.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}

func hasRoute(engine *gin.Engine, method, path string) bool {
	for _, route := range engine.Routes() {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}

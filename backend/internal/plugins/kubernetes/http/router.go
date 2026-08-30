package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const routePrefix = "/api/v1/plugins/kubernetes"

func RegisterRoutes(engine *gin.Engine) {
	if engine == nil {
		return
	}
	group := engine.Group(routePrefix)
	group.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}

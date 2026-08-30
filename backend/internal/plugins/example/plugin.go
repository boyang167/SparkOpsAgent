package example

import (
	"net/http"

	"github.com/gin-gonic/gin"

	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
)

type plugin struct{}

func init() {
	sparkplugin.Register(&plugin{})
}

func (p *plugin) Register(engine *gin.Engine) {
	g := engine.Group("/api/v1/plugins/example")
	g.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}

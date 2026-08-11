package httpx

import (
	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/docs"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/observability"
)

func NewEngine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(SecurityHeaders())
	r.Use(TraceID())
	r.Use(OperationLogMiddleware())
	r.Use(observability.MetricsMiddleware())
	r.GET("/health", func(c *gin.Context) {
		OK(c, map[string]string{"status": "up"})
	})
	observability.RegisterMetrics(r)
	docs.Register(r)
	return r
}

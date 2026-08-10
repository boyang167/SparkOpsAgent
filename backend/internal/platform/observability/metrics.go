package observability

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var httpRequestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	},
	[]string{"method", "path", "status"},
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
}

func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		status := strconv.Itoa(c.Writer.Status())
		httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
	}
}

func RegisterMetrics(r gin.IRoutes) {
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

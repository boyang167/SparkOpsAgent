package observability_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/observability"
	"github.com/stretchr/testify/require"
)

func TestRegisterMetrics_serves_prometheus_output(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(observability.MetricsMiddleware())
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	observability.RegisterMetrics(r)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "http_requests_total")
}

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

func TestGatherMetricsSummary_paginates_http_request_series(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(observability.MetricsMiddleware())
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/ready", func(c *gin.Context) { c.Status(http.StatusOK) })
	observability.RegisterMetrics(r)

	for _, path := range []string{"/health", "/ready"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	}

	page1 := observability.GatherMetricsSummary(1, 1)
	require.Equal(t, 1, page1.Page)
	require.Equal(t, 1, page1.PageSize)
	require.GreaterOrEqual(t, page1.Total, 2)
	require.Len(t, page1.HTTPRequestsTotal, 1)

	page2 := observability.GatherMetricsSummary(2, 1)
	require.Equal(t, 2, page2.Page)
	require.Len(t, page2.HTTPRequestsTotal, 1)
	require.NotEqual(t, page1.HTTPRequestsTotal[0].Label, page2.HTTPRequestsTotal[0].Label)

	lastPage := observability.GatherMetricsSummary(page1.Total, 1)
	require.Len(t, lastPage.HTTPRequestsTotal, 1)

	beyond := observability.GatherMetricsSummary(page1.Total+1, 1)
	require.Empty(t, beyond.HTTPRequestsTotal)
	require.Equal(t, page1.Total, beyond.Total)
}

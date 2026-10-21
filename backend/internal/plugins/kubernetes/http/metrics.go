package http

import (
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	k8smetrics "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/metrics"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type metricsHandler struct {
	clusterHandler
	client *k8smetrics.Client
}

func newMetricsHandler(ctx sparkplugin.PluginContext) metricsHandler {
	return metricsHandler{
		clusterHandler: newClusterHandler(ctx),
		client:         k8smetrics.NewClient(ctx.Endpoints),
	}
}

func (h metricsHandler) postMetrics(c *gin.Context) {
	if !h.readReady(c) {
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", stdhttp.StatusBadRequest)
		return
	}
	clusterID, err := kernel.ParseClusterInfoID(c.Param("clusterId"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_INVALID", "invalid cluster id", stdhttp.StatusBadRequest)
		return
	}
	if _, err := h.authorizedMeta(c, wsID, clusterID); err != nil {
		handleClusterError(c, err)
		return
	}
	var req k8smetrics.QueryRequest
	decoder := json.NewDecoder(stdhttp.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		httpx.Fail(c, "PLUGIN_METRICS_INVALID_REQUEST", "invalid metrics request", stdhttp.StatusBadRequest)
		return
	}
	query, err := k8smetrics.BuildRangeQuery(req, time.Now())
	if err != nil {
		handleMetricsError(c, err)
		return
	}
	settingsByCluster, err := h.store.List(c.Request.Context(), wsID, []string{clusterID.String()})
	if err != nil {
		handleClusterError(c, err)
		return
	}
	settings, ok := settingsByCluster[clusterID.String()]
	if !ok || settings.PrometheusURL == "" {
		httpx.Fail(c, "PLUGIN_PROMETHEUS_NOT_CONFIGURED", "prometheus is not configured", stdhttp.StatusBadRequest)
		return
	}
	data, err := h.client.Query(
		c.Request.Context(),
		settings.PrometheusURL,
		settings.PrometheusUser,
		settings.PrometheusPassword,
		query,
	)
	if err != nil {
		handleMetricsError(c, err)
		return
	}
	httpx.OK(c, data)
}

func handleMetricsError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, k8smetrics.ErrUnknownTemplate):
		httpx.Fail(c, "PLUGIN_METRICS_UNKNOWN_TEMPLATE", "unknown metrics template", stdhttp.StatusBadRequest)
	case errors.Is(err, k8smetrics.ErrInvalidRequest):
		httpx.Fail(c, "PLUGIN_METRICS_INVALID_REQUEST", "invalid metrics request", stdhttp.StatusBadRequest)
	default:
		httpx.Fail(c, "PLUGIN_PROMETHEUS_UPSTREAM", "prometheus upstream request failed", stdhttp.StatusBadGateway)
	}
}

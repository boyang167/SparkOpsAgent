package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/applog"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/observability"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/operationlog"
)

type ObservabilityHandler struct {
	logs      *applog.Store
	ops       *operationlog.Store
	cfg       *config.Config
}

func NewObservabilityHandler(logs *applog.Store, ops *operationlog.Store, cfg *config.Config) *ObservabilityHandler {
	return &ObservabilityHandler{logs: logs, ops: ops, cfg: cfg}
}

func (h *ObservabilityHandler) ListLogs(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	pageSize := queryIntObs(c, "page_size", applog.DefaultLimit)
	page := queryIntObs(c, "page", 1)
	filter := applog.ListFilter{
		WorkspaceID: wsID.String(),
		Level:       c.Query("level"),
		Category:    c.Query("category"),
		TraceID:     c.Query("trace_id"),
		Keyword:     c.Query("keyword"),
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
	}
	result, err := h.logs.List(c.Request.Context(), filter)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *ObservabilityHandler) ListLogFiles(c *gin.Context) {
	dir := observability.LogDirectory(h.cfg)
	files, err := observability.ListLogFiles(dir)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": files, "directory": dir})
}

func (h *ObservabilityHandler) TailLogFile(c *gin.Context) {
	dir := observability.LogDirectory(h.cfg)
	offset, _ := strconv.ParseInt(c.Query("offset"), 10, 64)
	limit := queryIntObs(c, "limit", 200)
	result, err := observability.TailLogFile(dir, c.Param("fileName"), offset, limit)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *ObservabilityHandler) GetTrace(c *gin.Context) {
	traceID := c.Param("traceId")
	logs, err := h.logs.ListByTrace(c.Request.Context(), traceID, applog.MaxLimit)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	var operations []operationlog.RecordView
	if h.ops != nil {
		wsID, ok := httpx.WorkspaceIDFromContext(c)
		if ok {
			opResult, err := h.ops.List(c.Request.Context(), operationlog.ListFilter{
				WorkspaceID: wsID.String(),
				TraceID:     traceID,
				Limit:       operationlog.MaxLimit,
			})
			if err == nil {
				operations = opResult.Items
			}
		}
	}
	httpx.OK(c, gin.H{
		"trace_id": traceID,
		"logs":     logs,
		"operations": operations,
	})
}

func (h *ObservabilityHandler) MetricsSummary(c *gin.Context) {
	pageSize := queryIntObs(c, "page_size", observability.DefaultMetricsPageSize)
	page := queryIntObs(c, "page", 1)
	summary := observability.GatherMetricsSummary(page, pageSize)
	httpx.OK(c, summary)
}

func queryIntObs(c *gin.Context, key string, fallback int) int {
	raw := c.Query(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

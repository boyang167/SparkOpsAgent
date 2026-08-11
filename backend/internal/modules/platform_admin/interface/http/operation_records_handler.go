package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/operationlog"
)

type OperationRecordsHandler struct {
	store *operationlog.Store
}

func NewOperationRecordsHandler(store *operationlog.Store) *OperationRecordsHandler {
	return &OperationRecordsHandler{store: store}
}

func (h *OperationRecordsHandler) List(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	pageSize := queryInt(c, "page_size", operationlog.DefaultLimit)
	page := queryInt(c, "page", 1)
	filter := operationlog.ListFilter{
		WorkspaceID: wsID.String(),
		IP:          c.Query("ip"),
		Method:      c.Query("method"),
		Path:        c.Query("path"),
		MemberID:    c.Query("member_id"),
		TraceID:     c.Query("trace_id"),
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
	}
	if statusStr := c.Query("status"); statusStr != "" {
		if status, err := strconv.Atoi(statusStr); err == nil {
			filter.Status = &status
		}
	}
	result, err := h.store.List(c.Request.Context(), filter)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *OperationRecordsHandler) Delete(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	if err := h.store.Delete(c.Request.Context(), c.Param("recordId"), wsID.String()); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

type batchDeleteOperationRecordsRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

func (h *OperationRecordsHandler) BatchDelete(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	var req batchDeleteOperationRecordsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	count, err := h.store.DeleteByIDs(c.Request.Context(), req.IDs, wsID.String())
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": count})
}

func queryInt(c *gin.Context, key string, fallback int) int {
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

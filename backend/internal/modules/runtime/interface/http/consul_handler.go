package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/consul"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type ConsulHandler struct {
	store consul.KVStore
}

func NewConsulHandler(store consul.KVStore) *ConsulHandler {
	return &ConsulHandler{store: store}
}

func (h *ConsulHandler) ListKV(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	prefix := c.Query("prefix")
	items, err := h.store.List(c.Request.Context(), wsID.String(), prefix)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *ConsulHandler) PutKV(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	var req struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.store.Put(c.Request.Context(), wsID.String(), req.Key, req.Value); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"saved": true})
}

func (h *ConsulHandler) DeleteKV(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	key := c.Query("key")
	if key == "" {
		httpx.Fail(c, "INVALID_REQUEST", "key required", http.StatusBadRequest)
		return
	}
	if err := h.store.Delete(c.Request.Context(), wsID.String(), key); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

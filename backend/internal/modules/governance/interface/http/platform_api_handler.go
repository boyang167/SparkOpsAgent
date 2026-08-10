package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/routes"
)

type PlatformAPIHandler struct {
	store  *persistence.APIRegistryStore
	engine *gin.Engine
}

func NewPlatformAPIHandler(store *persistence.APIRegistryStore, engine *gin.Engine) *PlatformAPIHandler {
	return &PlatformAPIHandler{store: store, engine: engine}
}

func (h *PlatformAPIHandler) List(c *gin.Context) {
	items, err := h.store.ListAll(c.Request.Context())
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *PlatformAPIHandler) SyncPreview(c *gin.Context) {
	registry := routes.Collect(h.engine)
	routeItems := make([]struct{ Method, Path string }, 0)
	for _, item := range registry.All() {
		routeItems = append(routeItems, struct{ Method, Path string }{Method: item.Method, Path: item.Path})
	}
	diff, err := h.store.Diff(c.Request.Context(), routeItems)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, diff)
}

type confirmSyncRequest struct {
	New    []persistence.RegisteredAPI `json:"new"`
	Delete []persistence.RegisteredAPI `json:"delete"`
}

func (h *PlatformAPIHandler) SyncConfirm(c *gin.Context) {
	var req confirmSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	diff := persistence.APISyncDiff{New: req.New, Delete: req.Delete}
	if err := h.store.ApplySync(c.Request.Context(), diff); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"synced": true})
}

func (h *PlatformAPIHandler) UpdateMeta(c *gin.Context) {
	id := c.Param("apiId")
	var body struct {
		APIGroup    string `json:"api_group"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.store.UpdateMeta(c.Request.Context(), id, body.APIGroup, body.Description); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": true})
}

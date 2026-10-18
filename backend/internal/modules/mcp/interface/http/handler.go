package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/application"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type Handler struct {
	rpc  *application.RPCHandler
	list *application.ListToolsHandler
}

func NewHandler(rpc *application.RPCHandler, list *application.ListToolsHandler) *Handler {
	return &Handler{rpc: rpc, list: list}
}

func (h *Handler) ListTools(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}

	items, err := h.list.Handle(c.Request.Context(), wsID, memberID)
	if err != nil {
		httpx.Fail(c, "MCP_LIST_TOOLS_FAILED", err.Error(), http.StatusInternalServerError)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) HandleRPC(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}

	var req application.RPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}

	resp := h.rpc.Handle(c.Request.Context(), wsID, memberID, req)
	if resp.Error != nil {
		c.JSON(http.StatusOK, resp)
		return
	}
	c.JSON(http.StatusOK, resp)
}

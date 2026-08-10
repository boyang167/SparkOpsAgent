package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/integration/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type Handler struct {
	listModels *query.ListModelsHandler
}

func NewHandler(listModels *query.ListModelsHandler) *Handler {
	return &Handler{listModels: listModels}
}

func (h *Handler) ListModels(c *gin.Context) {
	_, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	items, err := h.listModels.Handle(c.Request.Context())
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

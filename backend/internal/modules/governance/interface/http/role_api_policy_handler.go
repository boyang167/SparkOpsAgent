package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type RoleAPIPolicyHandler struct {
	list   *query.ListRoleAPIPoliciesHandler
	update *command.UpdateRoleAPIPoliciesHandler
}

func NewRoleAPIPolicyHandler(
	list *query.ListRoleAPIPoliciesHandler,
	update *command.UpdateRoleAPIPoliciesHandler,
) *RoleAPIPolicyHandler {
	return &RoleAPIPolicyHandler{list: list, update: update}
}

func (h *RoleAPIPolicyHandler) List(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	roleID, err := kernel.ParseRoleID(c.Param("roleId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid role id", http.StatusBadRequest)
		return
	}
	items, err := h.list.Handle(c.Request.Context(), wsID, roleID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *RoleAPIPolicyHandler) Update(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	roleID, err := kernel.ParseRoleID(c.Param("roleId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid role id", http.StatusBadRequest)
		return
	}
	var req struct {
		Items []persistence.RoleAPIPolicy `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.update.Handle(c.Request.Context(), wsID, roleID, req.Items); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": true})
}

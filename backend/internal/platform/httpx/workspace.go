package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const workspaceIDKey = "workspace_id"

func RequireWorkspace() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("X-Workspace-Id")
		if raw == "" {
			raw = c.Param("wsId")
		}
		if raw == "" {
			Fail(c, "TENANT_WORKSPACE_REQUIRED", "X-Workspace-Id header required", http.StatusBadRequest)
			c.Abort()
			return
		}
		wsID, err := kernel.ParseWorkspaceID(raw)
		if err != nil {
			Fail(c, "TENANT_WORKSPACE_INVALID", "invalid workspace id", http.StatusBadRequest)
			c.Abort()
			return
		}
		c.Set(workspaceIDKey, wsID)
		c.Next()
	}
}

func WorkspaceIDFromContext(c *gin.Context) (kernel.WorkspaceID, bool) {
	v, ok := c.Get(workspaceIDKey)
	if !ok {
		return kernel.WorkspaceID{}, false
	}
	id, ok := v.(kernel.WorkspaceID)
	return id, ok
}

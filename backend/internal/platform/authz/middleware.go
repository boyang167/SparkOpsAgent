package authz

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

func RequirePermission(checker PermissionChecker, perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID, ok := authn.MemberIDFromContext(c)
		if !ok {
			httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
			c.Abort()
			return
		}
		wsID, ok := httpx.WorkspaceIDFromContext(c)
		if !ok {
			httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
			c.Abort()
			return
		}
		if err := checker.RequirePermission(c.Request.Context(), wsID, memberID, perm); err != nil {
			httpx.HandleError(c, err)
			c.Abort()
			return
		}
		c.Next()
	}
}

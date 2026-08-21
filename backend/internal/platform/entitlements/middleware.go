package entitlements

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

// Middleware blocks workspace feature APIs when the module is disabled platform-wide.
func Middleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		moduleKey := APIPathEntitlement(path)
		if moduleKey == "" {
			c.Next()
			return
		}

		ents, err := ResolveCachedForWorkspace(c.Request.Context(), db, workspaceIDFromRequest(c))
		if err != nil {
			httpx.HandleError(c, err)
			c.Abort()
			return
		}
		if !ents.AllowsModule(moduleKey) {
			httpx.Fail(
				c,
				"ENTITLEMENT_MODULE_DISABLED",
				"feature module disabled: "+moduleKey,
				http.StatusForbidden,
			)
			c.Abort()
			return
		}
		c.Next()
	}
}

func workspaceIDFromRequest(c *gin.Context) string {
	if wsID, ok := httpx.WorkspaceIDFromContext(c); ok {
		return wsID.String()
	}
	raw := c.GetHeader("X-Workspace-Id")
	if raw == "" {
		raw = c.Param("wsId")
	}
	if raw == "" {
		return ""
	}
	wsID, err := kernel.ParseWorkspaceID(raw)
	if err != nil {
		return ""
	}
	return wsID.String()
}

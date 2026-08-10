package authz

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/policy"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

// WorkspaceAPIPolicyChecker reports whether a workspace uses role API policy enforcement.
type WorkspaceAPIPolicyChecker interface {
	HasAPIPoliciesForWorkspace(ctx context.Context, workspaceID kernel.WorkspaceID) (bool, error)
}

// APIAccessMiddleware enforces role-bound HTTP API policies via Casbin when configured.
// Skips routes without workspace context, platform/public/auth paths, and governance admins.
func APIAccessMiddleware(
	engine policy.Engine,
	permChecker PermissionChecker,
	policyChecker WorkspaceAPIPolicyChecker,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !shouldCheckAPIAccess(path) {
			c.Next()
			return
		}

		memberID, ok := authn.MemberIDFromContext(c)
		if !ok {
			c.Next()
			return
		}

		wsID, ok := workspaceIDFromRequest(c)
		if !ok {
			c.Next()
			return
		}

		if allowed, _ := permChecker.HasPermission(c.Request.Context(), wsID, memberID, PermGovernanceAdmin); allowed {
			c.Next()
			return
		}

		hasPolicies, err := policyChecker.HasAPIPoliciesForWorkspace(c.Request.Context(), wsID)
		if err != nil || !hasPolicies {
			c.Next()
			return
		}

		method := strings.ToUpper(c.Request.Method)
		allowed, err := engine.Enforce(c.Request.Context(), memberID.String(), wsID.String(), path, method)
		if err != nil {
			httpx.HandleError(c, err)
			c.Abort()
			return
		}
		if !allowed {
			httpx.Fail(c, "GOVERNANCE_API_ACCESS_DENIED", "api access denied by role policy", http.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

func shouldCheckAPIAccess(path string) bool {
	if !strings.HasPrefix(path, "/api/v1/") {
		return false
	}
	prefixes := []string{
		"/api/v1/auth/",
		"/api/v1/public/",
		"/api/v1/platform/",
		"/health",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return false
		}
	}
	return true
}

func workspaceIDFromRequest(c *gin.Context) (kernel.WorkspaceID, bool) {
	if wsID, ok := httpx.WorkspaceIDFromContext(c); ok {
		return wsID, true
	}
	raw := c.GetHeader("X-Workspace-Id")
	if raw == "" {
		raw = c.Param("wsId")
	}
	if raw == "" {
		return kernel.WorkspaceID{}, false
	}
	wsID, err := kernel.ParseWorkspaceID(raw)
	if err != nil {
		return kernel.WorkspaceID{}, false
	}
	return wsID, true
}

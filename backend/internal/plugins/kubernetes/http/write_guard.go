package http

import (
	"context"
	stdhttp "net/http"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func requireWorkspaceMember(members sparkplugin.WorkspaceMemberAuthorizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if members == nil {
			httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", stdhttp.StatusServiceUnavailable)
			c.Abort()
			return
		}
		memberID, ok := authn.MemberIDFromContext(c)
		if !ok {
			httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", stdhttp.StatusUnauthorized)
			c.Abort()
			return
		}
		wsID, ok := httpx.WorkspaceIDFromContext(c)
		if !ok {
			httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", stdhttp.StatusBadRequest)
			c.Abort()
			return
		}
		allowed, err := members.IsWorkspaceMember(c.Request.Context(), wsID, memberID)
		if err != nil {
			httpx.HandleError(c, err)
			c.Abort()
			return
		}
		if !allowed {
			httpx.Fail(c, "TENANT_NOT_WORKSPACE_MEMBER", "workspace membership required", stdhttp.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

func requireWrite(ctx context.Context, c *gin.Context, writer sparkplugin.WriteAuthorizer) (kernel.WorkspaceID, kernel.MemberID, bool) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", stdhttp.StatusBadRequest)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", stdhttp.StatusUnauthorized)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	allowed, err := writer.CanWrite(ctx, wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	if !allowed {
		httpx.Fail(c, "GOVERNANCE_API_ACCESS_DENIED", "governance admin permission required", stdhttp.StatusForbidden)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	return wsID, memberID, true
}

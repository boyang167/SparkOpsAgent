package authz

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkspaceMemberChecker interface {
	IsWorkspaceMember(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID) (bool, error)
}

type OrgWorkspaceCreatorChecker interface {
	CanCreateWorkspace(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) (bool, error)
}

func RequireWorkspaceMember(checker WorkspaceMemberChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if checker == nil {
			httpx.Fail(c, "TENANT_NOT_WORKSPACE_MEMBER", "workspace membership required", http.StatusForbidden)
			c.Abort()
			return
		}
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
		isMember, err := checker.IsWorkspaceMember(c.Request.Context(), wsID, memberID)
		if err != nil {
			httpx.HandleError(c, err)
			c.Abort()
			return
		}
		if !isMember {
			httpx.Fail(c, "TENANT_NOT_WORKSPACE_MEMBER", "workspace membership required", http.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

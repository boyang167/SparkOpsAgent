package authz

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const SystemRoleSuperAdmin = "super_admin"

type SystemRoleChecker interface {
	IsSuperAdmin(ctx context.Context, memberID kernel.MemberID) (bool, error)
}

func RequireSuperAdmin(checker SystemRoleChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if checker == nil {
			httpx.Fail(c, "AUTH_FORBIDDEN", "super admin required", http.StatusForbidden)
			c.Abort()
			return
		}
		memberID, ok := authn.MemberIDFromContext(c)
		if !ok {
			httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
			c.Abort()
			return
		}
		isSuper, err := checker.IsSuperAdmin(c.Request.Context(), memberID)
		if err != nil {
			httpx.HandleError(c, err)
			c.Abort()
			return
		}
		if !isSuper {
			httpx.Fail(c, "AUTH_FORBIDDEN", "super admin required", http.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

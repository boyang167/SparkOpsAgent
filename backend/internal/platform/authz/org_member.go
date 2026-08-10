package authz

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type OrgPermissionChecker interface {
	IsOrgMember(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) (bool, error)
	HasOrgPermission(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID, permission string) (bool, error)
}

func RequireOrgMember(checker OrgPermissionChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if checker == nil {
			httpx.Fail(c, "TENANT_ORG_ACCESS_DENIED", "organization membership required", http.StatusForbidden)
			c.Abort()
			return
		}
		memberID, ok := authn.MemberIDFromContext(c)
		if !ok {
			httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
			c.Abort()
			return
		}
		orgID, ok := httpx.OrgIDFromContext(c)
		if !ok {
			httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
			c.Abort()
			return
		}
		isMember, err := checker.IsOrgMember(c.Request.Context(), orgID, memberID)
		if err != nil {
			httpx.HandleError(c, err)
			c.Abort()
			return
		}
		if !isMember {
			httpx.Fail(c, "TENANT_ORG_ACCESS_DENIED", "organization membership required", http.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

func RequireOrgPermission(checker OrgPermissionChecker, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if checker == nil {
			httpx.Fail(c, "TENANT_ORG_ACCESS_DENIED", "organization permission required", http.StatusForbidden)
			c.Abort()
			return
		}
		memberID, ok := authn.MemberIDFromContext(c)
		if !ok {
			httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
			c.Abort()
			return
		}
		orgID, ok := httpx.OrgIDFromContext(c)
		if !ok {
			httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
			c.Abort()
			return
		}
		allowed, err := checker.HasOrgPermission(c.Request.Context(), orgID, memberID, permission)
		if err != nil {
			httpx.HandleError(c, err)
			c.Abort()
			return
		}
		if !allowed {
			httpx.Fail(c, "TENANT_ORG_ACCESS_DENIED", "organization permission required", http.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

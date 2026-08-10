package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const orgIDKey = "org_id"

func RequireOrg() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.Param("orgId")
		if raw == "" {
			Fail(c, "TENANT_ORG_REQUIRED", "organization id required", http.StatusBadRequest)
			c.Abort()
			return
		}
		orgID, err := kernel.ParseOrgID(raw)
		if err != nil {
			Fail(c, "TENANT_ORG_INVALID", "invalid organization id", http.StatusBadRequest)
			c.Abort()
			return
		}
		c.Set(orgIDKey, orgID)
		c.Next()
	}
}

func OrgIDFromContext(c *gin.Context) (kernel.OrgID, bool) {
	v, ok := c.Get(orgIDKey)
	if !ok {
		return kernel.OrgID{}, false
	}
	id, ok := v.(kernel.OrgID)
	return id, ok
}

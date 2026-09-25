package authn

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const memberIDKey = "auth_member_id"

func RequireAuth(tokens *TokenProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := extractBearer(c.GetHeader("Authorization"))
		if raw == "" {
			raw = strings.TrimSpace(c.Query("access_token"))
		}
		if raw == "" {
			httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
			c.Abort()
			return
		}
		memberID, err := tokens.ParseMemberID(raw)
		if err != nil {
			httpx.Fail(c, "AUTH_INVALID_TOKEN", "invalid or expired token", http.StatusUnauthorized)
			c.Abort()
			return
		}
		c.Set(memberIDKey, memberID)
		c.Next()
	}
}

func MemberIDFromContext(c *gin.Context) (kernel.MemberID, bool) {
	v, ok := c.Get(memberIDKey)
	if !ok {
		return kernel.MemberID{}, false
	}
	id, ok := v.(kernel.MemberID)
	return id, ok
}

func extractBearer(header string) string {
	if header == "" {
		return ""
	}
	const prefix = "Bearer "
	if strings.HasPrefix(header, prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return strings.TrimSpace(header)
}

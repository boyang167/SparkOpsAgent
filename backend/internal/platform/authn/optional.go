package authn

import (
	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

// OptionalAuth parses a bearer token when present and stores member id in context.
// Unlike RequireAuth it never aborts the request.
func OptionalAuth(tokens *TokenProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := MemberIDFromContext(c); ok {
			c.Next()
			return
		}
		raw := extractBearer(c.GetHeader("Authorization"))
		if raw == "" {
			c.Next()
			return
		}
		memberID, err := tokens.ParseMemberID(raw)
		if err != nil {
			c.Next()
			return
		}
		c.Set(memberIDKey, memberID)
		c.Next()
	}
}

// SetMemberIDForTest injects member id (used in tests).
func SetMemberIDForTest(c *gin.Context, id kernel.MemberID) {
	c.Set(memberIDKey, id)
}

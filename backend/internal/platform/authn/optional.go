package authn

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

// OptionalAuth parses a bearer token when present and stores member id in context.
// Unlike RequireAuth it never aborts the request.
func OptionalAuth(tokens *TokenProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := MemberIDFromContext(c); ok {
			if !validateAccessTokenQuery(c, tokens) {
				c.Abort()
				return
			}
			c.Next()
			return
		}
		queryToken := c.Query("access_token")
		if queryToken != "" {
			raw := extractStrictBearer(c.GetHeader("Authorization"))
			memberID, ok := parseMatchingQueryAndBearer(c, tokens, raw, queryToken)
			if !ok {
				c.Abort()
				return
			}
			c.Set(memberIDKey, memberID)
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

func validateAccessTokenQuery(c *gin.Context, tokens *TokenProvider) bool {
	queryToken := c.Query("access_token")
	if queryToken == "" {
		return true
	}
	raw := extractStrictBearer(c.GetHeader("Authorization"))
	_, ok := parseMatchingQueryAndBearer(c, tokens, raw, queryToken)
	return ok
}

func parseMatchingQueryAndBearer(c *gin.Context, tokens *TokenProvider, bearerToken, queryToken string) (kernel.MemberID, bool) {
	if tokens == nil || bearerToken == "" || queryToken == "" {
		httpx.Fail(c, "AUTH_INVALID_TOKEN", "invalid or expired token", http.StatusUnauthorized)
		return kernel.MemberID{}, false
	}
	bearerMemberID, err := tokens.ParseMemberID(bearerToken)
	if err != nil {
		httpx.Fail(c, "AUTH_INVALID_TOKEN", "invalid or expired token", http.StatusUnauthorized)
		return kernel.MemberID{}, false
	}
	queryMemberID, err := tokens.ParseMemberID(queryToken)
	if err != nil || queryMemberID != bearerMemberID {
		httpx.Fail(c, "AUTH_INVALID_TOKEN", "invalid or expired token", http.StatusUnauthorized)
		return kernel.MemberID{}, false
	}
	return bearerMemberID, true
}

func extractStrictBearer(header string) string {
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return ""
	}
	return fields[1]
}

// SetMemberIDForTest injects member id (used in tests).
func SetMemberIDForTest(c *gin.Context, id kernel.MemberID) {
	c.Set(memberIDKey, id)
}

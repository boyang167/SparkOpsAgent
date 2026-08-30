package authn

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

// RequireMemberForPluginAPIs rejects anonymous calls to plugin business routes.
// InstallAll registers those routes after OptionalAuth, so they would otherwise
// inherit the same skip-if-no-member behavior as APIAccessMiddleware.
func RequireMemberForPluginAPIs() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isPluginBusinessAPI(c.Request.URL.Path) {
			c.Next()
			return
		}
		if _, ok := MemberIDFromContext(c); !ok {
			httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
			c.Abort()
			return
		}
		c.Next()
	}
}

func isPluginBusinessAPI(path string) bool {
	rest, ok := strings.CutPrefix(path, "/api/v1/plugins/")
	if !ok || rest == "" {
		return false
	}
	id, suffix, cut := strings.Cut(rest, "/")
	return cut && id != "" && suffix != ""
}

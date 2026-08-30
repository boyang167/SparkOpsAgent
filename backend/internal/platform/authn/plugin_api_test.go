package authn_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestRequireMemberForPluginAPIsRejectsAnonymousPluginRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(authn.RequireMemberForPluginAPIs())
	engine.GET("/api/v1/plugins/example/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	engine.GET("/api/v1/admin/plugins", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/example/ping", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/plugins", nil))
	require.Equal(t, http.StatusOK, w.Code)

	engineAuthed := gin.New()
	engineAuthed.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, kernel.NewMemberID())
		c.Next()
	})
	engineAuthed.Use(authn.RequireMemberForPluginAPIs())
	engineAuthed.GET("/api/v1/plugins/example/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w = httptest.NewRecorder()
	engineAuthed.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/example/ping", nil))
	require.Equal(t, http.StatusOK, w.Code)
}

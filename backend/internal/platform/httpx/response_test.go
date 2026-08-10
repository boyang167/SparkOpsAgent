package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/stretchr/testify/require"
)

func TestHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := httpx.NewEngine()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var env httpx.Envelope
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&env))
	require.Equal(t, 0, env.Code)
	require.Equal(t, "ok", env.Message)
	require.NotEmpty(t, env.TraceID)
	require.Equal(t, "up", env.Data.(map[string]interface{})["status"])
}

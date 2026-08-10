package docs_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

func TestDocsRoutes(t *testing.T) {
	r := httpx.NewEngine()

	for _, path := range []string{"/api/docs", "/api/docs/openapi/openapi.yaml", "/api/docs/openapi/identity.yaml"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, rec.Code)
		}
	}
}

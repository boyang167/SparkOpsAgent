package filesystemgateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
)

func TestHTTPControlPortDecodesUploadGrantEnvelope(t *testing.T) {
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/internal/filesystem/v1/transfers/session-a/upload-grant", r.URL.Path)
		assert.Equal(t, "internal-secret", r.Header.Get("X-SparkOps-Internal-Token"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 0,
			"message": "ok",
			"data": {
				"temporary_object_key": "tmp/session-a",
				"expected_size": 12,
				"classification": "INTERNAL"
			}
		}`))
	}))
	defer control.Close()

	provider, err := local.New(filepath.Join(t.TempDir(), "storage"))
	require.NoError(t, err)
	defer provider.Close()
	port, err := NewHTTPControlPort(control.URL, "internal-secret", provider)
	require.NoError(t, err)

	grant, err := port.ResolveUpload(context.Background(), "session-a")
	require.NoError(t, err)
	assert.Equal(t, "tmp/session-a", grant.TemporaryObjectKey)
	assert.Equal(t, int64(12), grant.ExpectedSize)
	assert.Equal(t, model.TransferClassificationInternal, grant.Classification)
	assert.Same(t, provider, grant.Provider)
}

package filesystemgateway

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

func TestNewEngineKeepsLocalProviderOpenForUploads(t *testing.T) {
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "internal-secret", r.Header.Get("X-SparkOps-Internal-Token"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{
				"code": 0,
				"data": {
					"temporary_object_key": "tmp/session-bootstrap",
					"expected_size": 7,
					"classification": "INTERNAL"
				}
			}`))
		case r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"code":0,"data":{"recorded":true}}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer control.Close()

	cfg := &config.Config{}
	cfg.Filesystem.TransferGateway = config.TransferGatewayConfig{
		CapabilitySecret:   testCapabilitySecret,
		CapabilityAudience: testCapabilityAudience,
		MaxFileBytes:       1024,
		StorageRoot:        filepath.Join(t.TempDir(), "storage"),
		ControlBaseURL:     control.URL,
		InternalToken:      "internal-secret",
	}
	engine, err := NewEngineFromConfig(cfg)
	require.NoError(t, err)
	defer engine.Close()
	server := httptest.NewServer(engine)
	defer server.Close()

	req, err := http.NewRequest(
		http.MethodPut,
		server.URL+"/v1/transfers/session-bootstrap/content",
		bytes.NewBufferString("payload"),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+capabilityToken(t, capabilityOptions{
		sessionID: "session-bootstrap",
		action:    ActionTransferWrite,
	}))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

package filesystemgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
)

func TestUploadRejectsCapabilityForAnotherSession(t *testing.T) {
	h := newTestHarness(t, capabilityToken(t, capabilityOptions{
		sessionID: "session-a",
		action:    ActionTransferWrite,
	}))
	res := h.put("/v1/transfers/session-b/content", []byte("payload"))
	assert.Equal(t, http.StatusForbidden, res.Code)
}

func TestUploadRejectsOversizedBody(t *testing.T) {
	h := newTestHarness(t, capabilityToken(t, capabilityOptions{
		sessionID: "session-a",
		action:    ActionTransferWrite,
	}), withUploadGrant("session-a", UploadGrant{
		TemporaryObjectKey: "tmp/session-a",
		ExpectedSize:       4,
	}))
	res := h.put("/v1/transfers/session-a/content", []byte("toolarge"))
	assert.Equal(t, http.StatusRequestEntityTooLarge, res.Code)
}

func TestUploadStreamsAndReturnsChecksum(t *testing.T) {
	body := []byte("payload-bytes")
	h := newTestHarness(t, capabilityToken(t, capabilityOptions{
		sessionID: "session-x",
		action:    ActionTransferWrite,
	}), withUploadGrant("session-x", UploadGrant{
		TemporaryObjectKey: "tmp/session-x",
		ExpectedSize:       int64(len(body)),
	}))

	res := h.put("/v1/transfers/session-x/content", body)
	require.Equal(t, http.StatusOK, res.Code)

	var envelope struct {
		Data struct {
			Size   int64  `json:"size"`
			SHA256 string `json:"sha256"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &envelope))
	assert.Equal(t, int64(len(body)), envelope.Data.Size)
	assert.NotEmpty(t, envelope.Data.SHA256)

	info, err := h.provider.Stat(context.Background(), "tmp/session-x")
	require.NoError(t, err)
	assert.Equal(t, envelope.Data.SHA256, info.SHA256)
}

func TestDownloadRejectsCapabilityForAnotherFile(t *testing.T) {
	h := newTestHarness(t, capabilityToken(t, capabilityOptions{
		fileID: "file-a",
		action: ActionTransferRead,
	}))
	res := h.get("/v1/files/file-b/content")
	assert.Equal(t, http.StatusForbidden, res.Code)
}

func TestDownloadRestrictedFailsClosedWithoutAudit(t *testing.T) {
	h := newTestHarness(t, capabilityToken(t, capabilityOptions{
		fileID: "restricted-file",
		action: ActionTransferRead,
	}), withDownloadGrant("restricted-file", DownloadGrant{
		ObjectKey:      "objects/restricted",
		Classification: model.TransferClassificationRestricted,
		AuditOK:        false,
	}))
	res := h.get("/v1/files/restricted-file/content")
	assert.Equal(t, http.StatusForbidden, res.Code)
	assert.Empty(t, res.Body.String())
}

func TestDownloadStreamsObject(t *testing.T) {
	body := []byte("download-me")
	objectKey := "objects/file-1"
	h := newTestHarness(t, capabilityToken(t, capabilityOptions{
		fileID: "file-1",
		action: ActionTransferRead,
	}), withDownloadGrant("file-1", DownloadGrant{
		ObjectKey:      objectKey,
		Classification: model.TransferClassificationInternal,
		AuditOK:        true,
	}))
	_, err := h.provider.Put(context.Background(), storage.PutRequest{
		ObjectKey: objectKey,
		Body:      bytes.NewReader(body),
		Size:      int64(len(body)),
	})
	require.NoError(t, err)

	res := h.get("/v1/files/file-1/content")
	require.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, body, res.Body.Bytes())
}

func TestHealthAndReady(t *testing.T) {
	h := newTestHarness(t, "")
	health := h.get("/health")
	require.Equal(t, http.StatusOK, health.Code)
	ready := h.get("/ready")
	require.Equal(t, http.StatusOK, ready.Code)
}

type testHarness struct {
	t        *testing.T
	server   *httptest.Server
	token    string
	provider storage.Provider
}

type harnessOption func(*mockControlPort)

func withUploadGrant(sessionID string, grant UploadGrant) harnessOption {
	return func(port *mockControlPort) {
		grant.Provider = port.provider
		port.uploads[sessionID] = grant
	}
}

func withDownloadGrant(fileID string, grant DownloadGrant) harnessOption {
	return func(port *mockControlPort) {
		grant.Provider = port.provider
		port.downloads[fileID] = grant
	}
}

func newTestHarness(t *testing.T, token string, opts ...harnessOption) *testHarness {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)

	root := t.TempDir()
	providerImpl, err := local.New(filepath.Join(root, "storage"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = providerImpl.Close() })
	var provider storage.Provider = providerImpl

	port := &mockControlPort{
		provider:  provider,
		uploads:   map[string]UploadGrant{},
		downloads: map[string]DownloadGrant{},
	}
	for _, opt := range opts {
		opt(port)
	}

	server, err := New(Options{
		CapabilitySecret:   testCapabilitySecret,
		CapabilityAudience: testCapabilityAudience,
		MaxFileBytes:       20 * 1024 * 1024 * 1024,
		Control:            port,
		NonceStore:         NewMemoryNonceStore(),
	})
	require.NoError(t, err)

	engine := gin.New()
	engine.Use(gin.Recovery())
	server.RegisterRoutes(engine)
	httpServer := httptest.NewServer(engine)
	t.Cleanup(httpServer.Close)

	return &testHarness{t: t, server: httpServer, token: token, provider: provider}
}

func (h *testHarness) put(path string, body []byte) *httptest.ResponseRecorder {
	req, err := http.NewRequest(http.MethodPut, h.server.URL+path, bytes.NewReader(body))
	require.NoError(h.t, err)
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(h.t, err)
	defer resp.Body.Close()
	rec := httptest.NewRecorder()
	rec.Code = resp.StatusCode
	rec.Body, err = readBody(resp.Body)
	require.NoError(h.t, err)
	for k, values := range resp.Header {
		for _, v := range values {
			rec.Header().Add(k, v)
		}
	}
	return rec
}

func (h *testHarness) get(path string) *httptest.ResponseRecorder {
	req, err := http.NewRequest(http.MethodGet, h.server.URL+path, nil)
	require.NoError(h.t, err)
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(h.t, err)
	defer resp.Body.Close()
	rec := httptest.NewRecorder()
	rec.Code = resp.StatusCode
	rec.Body, err = readBody(resp.Body)
	require.NoError(h.t, err)
	return rec
}

func readBody(r io.Reader) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}
	return &buf, nil
}

type mockControlPort struct {
	provider  storage.Provider
	uploads   map[string]UploadGrant
	downloads map[string]DownloadGrant
}

func (m *mockControlPort) ResolveUpload(_ context.Context, sessionID string) (UploadGrant, error) {
	grant, ok := m.uploads[sessionID]
	if !ok {
		return UploadGrant{}, fmt.Errorf("session %s not found", sessionID)
	}
	if grant.Provider == nil {
		grant.Provider = m.provider
	}
	return grant, nil
}

func (m *mockControlPort) ResolveDownload(_ context.Context, fileID string) (DownloadGrant, error) {
	grant, ok := m.downloads[fileID]
	if !ok {
		return DownloadGrant{}, fmt.Errorf("file %s not found", fileID)
	}
	if grant.Provider == nil {
		grant.Provider = m.provider
	}
	return grant, nil
}

package operationlog

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMiddlewareDoesNotTruncateMultipartBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := openOperationLogStore(t, "oplog_multipart")

	var received []byte
	engine := gin.New()
	engine.Use(Middleware(store))
	engine.POST("/api/v1/admin/plugins/install", func(c *gin.Context) {
		raw, readErr := io.ReadAll(c.Request.Body)
		require.NoError(t, readErr)
		received = raw
		c.Status(http.StatusOK)
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "plugin.zip")
	require.NoError(t, err)
	payload := bytes.Repeat([]byte("abcdefghij"), 1000) // 10 KiB > maxBodyBytes
	_, err = part.Write(payload)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/plugins/install", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Greater(t, len(received), maxBodyBytes)
	require.Contains(t, string(received), "abcdefghij")

	records := waitForOperationLogRecords(t, store, 1)
	require.Equal(t, "[multipart omitted]", records[0].Body)
	require.Empty(t, records[0].Resp)
	require.NotContains(t, records[0].Body, "abcdefghij")
	require.NotContains(t, records[0].Resp, "abcdefghij")
}

func TestMiddlewareOmitsEventStreamResponseBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := openOperationLogStore(t, "oplog_sse")
	engine := gin.New()
	engine.Use(Middleware(store))
	engine.GET("/api/v1/plugins/kubernetes/clusters/c1/namespaces/default/pods/p1/logs", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream; charset=utf-8")
		_, _ = c.Writer.Write([]byte("event: log\ndata: \"sse-secret\"\n\n"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/kubernetes/clusters/c1/namespaces/default/pods/p1/logs?access_token=sse-secret&command=cat", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "sse-secret")
	records := waitForOperationLogRecords(t, store, 1)
	require.Equal(t, "/api/v1/plugins/kubernetes/clusters/c1/namespaces/default/pods/p1/logs", records[0].Path)
	require.Empty(t, records[0].Body)
	require.Equal(t, "[stream omitted]", records[0].Resp)
	require.NotContains(t, records[0].Path, "access_token")
	require.NotContains(t, records[0].Path, "command")
	require.NotContains(t, records[0].Resp, "sse-secret")
}

func TestMiddlewareOmitsKubernetesPluginRequestBodyAndPreservesHandlerBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := openOperationLogStore(t, "oplog_plugin_body")
	engine := gin.New()
	engine.Use(Middleware(store))
	received := map[string]string{}
	chatBody := &countingReadCloser{Reader: bytes.NewBufferString(`{"prompt":"chat-secret"}`)}
	engine.POST("/api/v1/plugins/kubernetes/clusters/c1/chat/query", func(c *gin.Context) {
		require.Zero(t, chatBody.reads, "middleware must not pre-read kubernetes plugin body")
		raw, readErr := io.ReadAll(c.Request.Body)
		require.NoError(t, readErr)
		received["chat"] = string(raw)
		c.Status(http.StatusOK)
	})
	engine.PUT("/api/v1/plugins/kubernetes/clusters/c1/settings", func(c *gin.Context) {
		raw, readErr := io.ReadAll(c.Request.Body)
		require.NoError(t, readErr)
		received["settings"] = string(raw)
		c.Status(http.StatusOK)
	})

	payloads := []struct {
		method string
		path   string
		key    string
		body   io.ReadCloser
		want   string
	}{
		{http.MethodPost, "/api/v1/plugins/kubernetes/clusters/c1/chat/query", "chat", chatBody, `{"prompt":"chat-secret"}`},
		{http.MethodPut, "/api/v1/plugins/kubernetes/clusters/c1/settings", "settings", io.NopCloser(bytes.NewBufferString(`{"llm_api_key":"settings-secret","velaux_password":"velaux-secret"}`)), `{"llm_api_key":"settings-secret","velaux_password":"velaux-secret"}`},
	}
	for _, payload := range payloads {
		req := httptest.NewRequest(payload.method, payload.path, nil)
		req.Body = payload.body
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, payload.want, received[payload.key])
	}
	require.Positive(t, chatBody.reads)

	records := waitForOperationLogRecords(t, store, 2)
	encoded, err := json.Marshal(records)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "[plugin body omitted]")
	for _, secret := range []string{"chat-secret", "settings-secret", "velaux-secret"} {
		require.NotContains(t, string(encoded), secret)
	}
}

func TestMiddlewarePreservesLargeJSONBodyForHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := openOperationLogStore(t, "oplog_large_json")

	var received []byte
	engine := gin.New()
	engine.Use(Middleware(store))
	engine.POST("/api/v1/workspaces/ws-1/clusters", func(c *gin.Context) {
		raw, readErr := io.ReadAll(c.Request.Body)
		require.NoError(t, readErr)
		received = raw
		c.Status(http.StatusOK)
	})

	payload := `{"cluster_name":"dev","kube_config":"` + strings.Repeat("A", maxBodyBytes+2048) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-1/clusters", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, payload, string(received))

	records := waitForOperationLogRecords(t, store, 1)
	require.LessOrEqual(t, len(records[0].Body), maxBodyBytes+len("...[truncated]"))
	require.True(t, strings.HasPrefix(records[0].Body, `{"cluster_name":"dev"`))
	require.Contains(t, records[0].Body, "...[truncated]")
}

type countingReadCloser struct {
	io.Reader
	reads int
}

func (r *countingReadCloser) Read(p []byte) (int, error) {
	r.reads++
	return r.Reader.Read(p)
}

func (r *countingReadCloser) Close() error { return nil }

func openOperationLogStore(t *testing.T, name string) *Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	store := NewStore(db)
	require.NoError(t, store.AutoMigrate())
	return store
}

func waitForOperationLogRecords(t *testing.T, store *Store, want int) []RecordView {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		result, err := store.List(context.Background(), ListFilter{Limit: 10})
		require.NoError(t, err)
		if len(result.Items) >= want {
			return result.Items
		}
		if time.Now().After(deadline) {
			require.Len(t, result.Items, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

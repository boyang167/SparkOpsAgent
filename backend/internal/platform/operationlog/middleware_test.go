package operationlog

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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

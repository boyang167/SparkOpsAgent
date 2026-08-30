package records

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestListRecordsScopesWorkspaceSortsAndOmitsBlob(t *testing.T) {
	db := openRecordsDB(t)
	wsID := kernel.NewWorkspaceID()
	otherWS := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	insertRecord(t, db, k8sschema.Record{
		ID:          "old",
		WorkspaceID: wsID.String(),
		ClusterID:   clusterID.String(),
		Kind:        "exec",
		StartedAt:   time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC),
		Blob:        []byte("old blob"),
	})
	insertRecord(t, db, k8sschema.Record{
		ID:          "new",
		WorkspaceID: wsID.String(),
		ClusterID:   clusterID.String(),
		Kind:        "exec",
		StartedAt:   time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC),
		Blob:        []byte("secret blob"),
	})
	insertRecord(t, db, k8sschema.Record{
		ID:          "other",
		WorkspaceID: otherWS.String(),
		ClusterID:   clusterID.String(),
		Kind:        "exec",
		StartedAt:   time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC),
		Blob:        []byte("other secret"),
	})
	engine := recordsTestEngine(memberID, sparkplugin.PluginContext{
		DB:      db,
		Members: allowRecordMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	rec := performRecords(engine, http.MethodGet, "/records?page=1&pageSize=100", wsID)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.NotContains(t, rec.Body.String(), "secret blob")
	require.NotContains(t, rec.Body.String(), "other secret")
	var body struct {
		Items []struct {
			ID          string `json:"id"`
			WorkspaceID string `json:"workspaceId"`
		} `json:"items"`
		Total int64 `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, int64(2), body.Total)
	require.Equal(t, []string{"new", "old"}, []string{body.Items[0].ID, body.Items[1].ID})
	require.Equal(t, wsID.String(), body.Items[0].WorkspaceID)
}

func TestGetRecordReturnsNDJSONOnlyForSameWorkspace(t *testing.T) {
	db := openRecordsDB(t)
	wsID := kernel.NewWorkspaceID()
	otherWS := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	insertRecord(t, db, k8sschema.Record{
		ID:          "same",
		WorkspaceID: wsID.String(),
		ClusterID:   kernel.NewClusterInfoID().String(),
		Kind:        "exec",
		StartedAt:   time.Now().UTC(),
		Blob:        []byte("{\"time\":0,\"stream\":\"stdout\",\"data\":\"ok\"}\n"),
	})
	insertRecord(t, db, k8sschema.Record{
		ID:          "other",
		WorkspaceID: otherWS.String(),
		ClusterID:   kernel.NewClusterInfoID().String(),
		Kind:        "exec",
		StartedAt:   time.Now().UTC(),
		Blob:        []byte("other secret\n"),
	})
	engine := recordsTestEngine(memberID, sparkplugin.PluginContext{
		DB:      db,
		Members: allowRecordMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	ok := performRecords(engine, http.MethodGet, "/records/same", wsID)
	cross := performRecords(engine, http.MethodGet, "/records/other", wsID)
	unknown := performRecords(engine, http.MethodGet, "/records/missing", wsID)

	require.Equal(t, http.StatusOK, ok.Code)
	require.Equal(t, "application/x-ndjson", ok.Header().Get("Content-Type"))
	require.Equal(t, "no-store", ok.Header().Get("Cache-Control"))
	require.Equal(t, "{\"time\":0,\"stream\":\"stdout\",\"data\":\"ok\"}\n", ok.Body.String())
	require.Equal(t, http.StatusNotFound, cross.Code)
	require.Equal(t, http.StatusNotFound, unknown.Code)
	require.NotContains(t, cross.Body.String(), "other secret")
}

func TestDeleteRecordRequiresWriteBeforeDeleting(t *testing.T) {
	db := openRecordsDB(t)
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	insertRecord(t, db, k8sschema.Record{
		ID:          "record",
		WorkspaceID: wsID.String(),
		ClusterID:   kernel.NewClusterInfoID().String(),
		Kind:        "exec",
		StartedAt:   time.Now().UTC(),
		Blob:        []byte("recording"),
	})
	writer := &recordingRecordWriter{allow: false}
	engine := recordsTestEngine(memberID, sparkplugin.PluginContext{
		DB:      db,
		Writer:  writer,
		Members: allowRecordMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	rec := performRecords(engine, http.MethodDelete, "/records/record", wsID)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.True(t, writer.called)
	var count int64
	require.NoError(t, db.Model(&k8sschema.Record{}).Where("id = ?", "record").Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestDeleteRecordOnlyDeletesSameWorkspace(t *testing.T) {
	db := openRecordsDB(t)
	wsID := kernel.NewWorkspaceID()
	otherWS := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	insertRecord(t, db, k8sschema.Record{
		ID:          "other",
		WorkspaceID: otherWS.String(),
		ClusterID:   kernel.NewClusterInfoID().String(),
		Kind:        "exec",
		StartedAt:   time.Now().UTC(),
		Blob:        []byte("other secret"),
	})
	engine := recordsTestEngine(memberID, sparkplugin.PluginContext{
		DB:      db,
		Writer:  &recordingRecordWriter{allow: true},
		Members: allowRecordMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	rec := performRecords(engine, http.MethodDelete, "/records/other", wsID)

	require.Equal(t, http.StatusNotFound, rec.Code)
	var count int64
	require.NoError(t, db.Model(&k8sschema.Record{}).Where("id = ?", "other").Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestRecordPaginationRejectsUnsafeValues(t *testing.T) {
	db := openRecordsDB(t)
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	engine := recordsTestEngine(memberID, sparkplugin.PluginContext{
		DB:      db,
		Members: allowRecordMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	for _, query := range []string{
		"/records?page=0",
		"/records?page=1&pageSize=0",
		"/records?page=1&pageSize=101",
		"/records?page=9007199254740993&pageSize=10",
		"/records?page=1.5&pageSize=10",
	} {
		rec := performRecords(engine, http.MethodGet, query, wsID)
		require.Equal(t, http.StatusBadRequest, rec.Code, query)
	}
}

func TestSessionRecorderPersistsBoundedJSONLinesAndTruncates(t *testing.T) {
	db := openRecordsDB(t)
	store := NewStore(db)
	wsID := kernel.NewWorkspaceID()
	clusterID := kernel.NewClusterInfoID()
	recorder, err := store.Start(context.Background(), Session{
		WorkspaceID: wsID.String(),
		ClusterID:   clusterID.String(),
		Kind:        "exec",
		StartedAt:   time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	_, err = recorder.Stdout().Write(bytes.Repeat([]byte("a"), MaxRecordingBytes+1024))
	require.NoError(t, err)
	_, err = recorder.Stderr().Write([]byte("secret stderr after limit"))
	require.NoError(t, err)
	require.NoError(t, recorder.Close(context.Background(), time.Date(2026, 8, 30, 12, 1, 0, 0, time.UTC)))

	var saved k8sschema.Record
	require.NoError(t, db.First(&saved, "id = ?", recorder.ID()).Error)
	require.NotNil(t, saved.EndedAt)
	require.LessOrEqual(t, len(saved.Blob), MaxRecordingBytes+256)
	require.Contains(t, string(saved.Blob), `"stream":"stdout"`)
	require.Contains(t, string(saved.Blob), `"stream":"truncated"`)
	require.NotContains(t, string(saved.Blob), "secret stderr after limit")
	for _, line := range strings.Split(strings.TrimSpace(string(saved.Blob)), "\n") {
		var frame map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &frame), line)
	}
}

func openRecordsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS plugin_kubernetes").Error)
	require.NoError(t, k8sschema.AutoMigrate(db))
	return db
}

func recordsTestEngine(memberID kernel.MemberID, ctx sparkplugin.PluginContext) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	engine.Use(httpx.RequireWorkspace())
	engine.Use(func(c *gin.Context) {
		if ctx.Members == nil {
			c.Next()
			return
		}
		wsID, _ := httpx.WorkspaceIDFromContext(c)
		allowed, _ := ctx.Members.IsWorkspaceMember(c.Request.Context(), wsID, memberID)
		if !allowed {
			httpx.Fail(c, "TENANT_NOT_WORKSPACE_MEMBER", "workspace membership required", http.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	})
	handler := NewHandler(ctx)
	engine.GET("/records", handler.List)
	engine.GET("/records/:recordId", handler.Get)
	engine.DELETE("/records/:recordId", handler.Delete)
	return engine
}

func performRecords(engine *gin.Engine, method, path string, wsID kernel.WorkspaceID) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Workspace-Id", wsID.String())
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func insertRecord(t *testing.T, db *gorm.DB, record k8sschema.Record) {
	t.Helper()
	require.NoError(t, db.Create(&record).Error)
}

type allowRecordMembers struct {
	grants map[kernel.WorkspaceID]map[kernel.MemberID]bool
}

func (m allowRecordMembers) IsWorkspaceMember(_ context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) (bool, error) {
	return m.grants[wsID][memberID], nil
}

type recordingRecordWriter struct {
	allow  bool
	called bool
}

func (w *recordingRecordWriter) CanWrite(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	w.called = true
	return w.allow, nil
}

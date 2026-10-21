package diagnosis

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	k8sresource "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/resource"
	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestArthasBootJarPathUsesBundledArchiveLayout(t *testing.T) {
	path, err := arthasBootJarPath(k8sresource.ArthasArchive, "/tmp/session")

	require.NoError(t, err)
	require.Equal(t, "/tmp/session/arthas-bin/arthas-boot.jar", path)
}

func TestStartExtractsArchiveWithSeparateArgvAndStoresSession(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openDiagnosisDB(t)
	exec := &recordingExecutor{}
	now := time.Unix(1_700_000_000, 0).UTC()
	engine := diagnosisEngine(memberID, NewHandlerWithOptions(sparkplugin.PluginContext{
		DB:       db,
		Clusters: diagnosisClusterFactory{clusterID: clusterID},
		Writer:   allowWriter{allow: true},
	}, Options{
		Executor: exec,
		Archive:  testArthasArchive(t),
		Now:      func() time.Time { return now },
		RunAsync: runSync,
	}))

	rec := performJSON(engine, http.MethodPost, "/clusters/"+clusterID.String()+"/diagnosis", wsID, map[string]any{
		"namespace": "default",
		"pod":       "api-0",
		"container": "app",
		"pid":       1234,
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var env httpx.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	body, err := json.Marshal(env.Data)
	require.NoError(t, err)
	require.Contains(t, string(body), `"status":"ready"`)

	require.Len(t, exec.calls, 2)
	require.Equal(t, []string{"mkdir", "-p", exec.calls[0].Command[2]}, exec.calls[0].Command)
	require.Equal(t, []string{"tar", "-xmf", "-", "-C", exec.calls[0].Command[2]}, exec.calls[1].Command)
	require.NotContains(t, strings.Join(exec.calls[0].Command, " "), "/bin/sh")
	require.NotNil(t, exec.calls[1].Stdin)
	tarPayload, err := io.ReadAll(exec.calls[1].Stdin)
	require.NoError(t, err)
	require.Contains(t, string(tarPayload), "jar")

	var row k8sschema.Diagnosis
	require.NoError(t, db.First(&row).Error)
	require.Equal(t, "ready", row.Status)
	require.Equal(t, "default", row.Namespace)
	require.Equal(t, "api-0", row.Pod)
	require.Equal(t, "app", row.Container)
	require.Equal(t, 1234, row.PID)
	require.Equal(t, now, row.CreatedAt)
	require.Equal(t, now.Add(DefaultSessionTTL), row.ExpiresAt)
}

func TestStartCleansSessionWhenExtractFails(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openDiagnosisDB(t)
	exec := &recordingExecutor{failCommand: "tar"}
	engine := diagnosisEngine(memberID, NewHandlerWithOptions(sparkplugin.PluginContext{
		DB:       db,
		Clusters: diagnosisClusterFactory{clusterID: clusterID},
		Writer:   allowWriter{allow: true},
	}, Options{
		Executor: exec,
		Archive:  testArthasArchive(t),
		RunAsync: runSync,
	}))

	rec := performJSON(engine, http.MethodPost, "/clusters/"+clusterID.String()+"/diagnosis", wsID, map[string]any{
		"namespace": "default",
		"pod":       "api-0",
		"container": "app",
		"pid":       1234,
	})

	require.Equal(t, http.StatusBadGateway, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "PLUGIN_ARTHAS_SESSION_START_FAILED")
	require.True(t, slices.ContainsFunc(exec.calls, func(call execCall) bool {
		return slices.Equal(call.Command[:3], []string{"rm", "-rf", "--"})
	}))
	var row k8sschema.Diagnosis
	require.NoError(t, db.First(&row).Error)
	require.Equal(t, "failed", row.Status)
}

func TestStreamRequiresAdminBeforeStartingArthas(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openDiagnosisDB(t)
	require.NoError(t, db.Create(&k8sschema.Diagnosis{
		ID:          kernel.NewPluginInstallID().String(),
		WorkspaceID: wsID.String(),
		ClusterID:   clusterID.String(),
		Namespace:   "default",
		Pod:         "api-0",
		Container:   "app",
		PID:         1234,
		SessionDir:  "/tmp/session",
		Status:      "ready",
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
	}).Error)
	exec := &recordingExecutor{}
	engine := diagnosisEngine(memberID, NewHandlerWithOptions(sparkplugin.PluginContext{
		DB:       db,
		Clusters: diagnosisClusterFactory{clusterID: clusterID},
		Writer:   allowWriter{allow: false},
	}, Options{Executor: exec, RunAsync: runSync}))

	rec := performJSON(engine, http.MethodGet, "/diagnosis/"+kernel.NewPluginInstallID().String()+"/ws", wsID, nil)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Empty(t, exec.calls)
}

func TestStreamRunsArthasJarAndCleansAfterExit(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openDiagnosisDB(t)
	diagnosisID := kernel.NewPluginInstallID().String()
	require.NoError(t, db.Create(&k8sschema.Diagnosis{
		ID:          diagnosisID,
		WorkspaceID: wsID.String(),
		ClusterID:   clusterID.String(),
		Namespace:   "default",
		Pod:         "api-0",
		Container:   "app",
		PID:         1234,
		SessionDir:  "/tmp/session",
		JarPath:     "/tmp/session/arthas-bin/arthas-boot.jar",
		Status:      "ready",
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
	}).Error)
	exec := &recordingExecutor{stdout: "started\n"}
	engine := diagnosisEngine(memberID, NewHandlerWithOptions(sparkplugin.PluginContext{
		DB:       db,
		Clusters: diagnosisClusterFactory{clusterID: clusterID},
		Writer:   allowWriter{allow: true},
	}, Options{Executor: exec, RunAsync: runSync}))

	rec := performJSON(engine, http.MethodGet, "/diagnosis/"+diagnosisID+"/ws", wsID, nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "event: output")
	require.True(t, slices.ContainsFunc(exec.calls, func(call execCall) bool {
		return slices.Equal(call.Command, []string{"java", "-jar", "/tmp/session/arthas-bin/arthas-boot.jar", "1234"})
	}))
	require.True(t, slices.ContainsFunc(exec.calls, func(call execCall) bool {
		return slices.Equal(call.Command, []string{"rm", "-rf", "--", "/tmp/session"})
	}))
}

func testArthasArchive(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "arthas-bin/arthas-boot.jar", Mode: 0o644, Size: 3}))
	_, err := tw.Write([]byte("jar"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	return buf.Bytes()
}

func openDiagnosisDB(t *testing.T) *gorm.DB {
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

func diagnosisEngine(memberID kernel.MemberID, handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	engine.Use(httpx.RequireWorkspace())
	engine.POST("/clusters/:clusterId/diagnosis", handler.Start)
	engine.GET("/diagnosis/:id/ws", handler.Stream)
	engine.DELETE("/diagnosis/:id", handler.Delete)
	return engine
}

func performJSON(engine *gin.Engine, method, path string, wsID kernel.WorkspaceID, payload any) *httptest.ResponseRecorder {
	var body bytes.Buffer
	if payload != nil {
		_ = json.NewEncoder(&body).Encode(payload)
	}
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("X-Workspace-Id", wsID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

type recordingExecutor struct {
	calls       []execCall
	failCommand string
	stdout      string
}

func (e *recordingExecutor) Exec(_ context.Context, _ kubernetes.Interface, _ *rest.Config, req ExecRequest) error {
	e.calls = append(e.calls, execCall{Command: append([]string(nil), req.Command...), Stdin: req.Stdin})
	if len(req.Command) > 0 && req.Command[0] == e.failCommand {
		return errors.New("boom")
	}
	if req.Stdout != nil && e.stdout != "" {
		_, _ = io.WriteString(req.Stdout, e.stdout)
	}
	return nil
}

type execCall struct {
	Command []string
	Stdin   io.Reader
}

type diagnosisClusterFactory struct {
	clusterID kernel.ClusterInfoID
}

func (f diagnosisClusterFactory) ForWorkspace(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	return fake.NewSimpleClientset(), nil, &rest.Config{Host: "https://cluster.example"}, &sparkplugin.ClusterMeta{ID: f.clusterID.String(), Name: "prod"}, nil
}

func (f diagnosisClusterFactory) ListMeta(context.Context, kernel.WorkspaceID) ([]sparkplugin.ClusterMeta, error) {
	return []sparkplugin.ClusterMeta{{ID: f.clusterID.String(), Name: "prod"}}, nil
}

type allowWriter struct {
	allow bool
}

func (w allowWriter) CanWrite(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return w.allow, nil
}

func runSync(fn func()) {
	fn()
}

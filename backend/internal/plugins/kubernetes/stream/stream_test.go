package stream

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestLogsStreamsEachLineAsJSONSSE(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	logs := &fakeLogStreamer{body: "alpha\nbeta\n"}
	engine := streamTestEngine(memberID, sparkplugin.PluginContext{
		Clusters: fakeStreamClusterFactory{clusterID: clusterID},
	}, logs, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/clusters/"+clusterID.String()+"/namespaces/default/pods/web-0/logs?container=app&tailLines=10&sinceSeconds=60", nil)
	req.Header.Set("X-Workspace-Id", wsID.String())
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
	require.Equal(t, "no-cache, no-transform", rec.Header().Get("Cache-Control"))
	require.Equal(t, "no", rec.Header().Get("X-Accel-Buffering"))
	require.Equal(t, LogRequest{
		Namespace:    "default",
		Pod:          "web-0",
		Container:    "app",
		TailLines:    int64Ptr(10),
		SinceSeconds: int64Ptr(60),
	}, logs.request)
	body := rec.Body.String()
	require.Contains(t, body, "event: open\n")
	require.Contains(t, body, "event: log\ndata: \"alpha\\n\"\n\n")
	require.Contains(t, body, "event: log\ndata: \"beta\\n\"\n\n")
}

func TestExecRequiresWriterBeforeClusterConnection(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	factory := &recordingStreamClusterFactory{}
	exec := &fakeExecStreamer{}
	engine := streamTestEngine(memberID, sparkplugin.PluginContext{
		Clusters: factory,
		Writer:   allowStreamWriter{allow: false},
	}, nil, exec)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/clusters/"+clusterID.String()+"/namespaces/default/pods/web-0/exec?command=date", nil)
	req.Header.Set("X-Workspace-Id", wsID.String())
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.False(t, factory.called)
	require.False(t, exec.called)
}

func TestExecStreamsOutputErrorExitAndAuditsEnd(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	audit := &recordingAudit{}
	exec := &fakeExecStreamer{
		run: func(ctx context.Context, req ExecRequest) error {
			_, _ = req.Stdout.Write([]byte("hello\n"))
			_, _ = req.Stderr.Write([]byte("warn\n"))
			return nil
		},
	}
	engine := streamTestEngine(memberID, sparkplugin.PluginContext{
		Clusters: fakeStreamClusterFactory{clusterID: clusterID},
		Writer:   allowStreamWriter{allow: true},
		Audit:    audit,
	}, nil, exec)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/clusters/"+clusterID.String()+"/namespaces/default/pods/web-0/exec?container=app&command=echo&command=hello", nil)
	req.Header.Set("X-Workspace-Id", wsID.String())
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, ExecRequest{
		Namespace: "default",
		Pod:       "web-0",
		Container: "app",
		Command:   []string{"echo", "hello"},
	}, exec.request.withoutWriters())
	body := rec.Body.String()
	require.Contains(t, body, "event: open\n")
	require.Contains(t, body, "event: output\ndata: \"hello\\n\"\n\n")
	require.Contains(t, body, "event: error\ndata: \"warn\\n\"\n\n")
	require.Contains(t, body, "event: exit\ndata: {\"code\":0}\n\n")
	require.Len(t, audit.events, 2)
	require.Equal(t, "kubernetes.exec.started", audit.events[0].Action)
	require.Equal(t, "kubernetes.exec.ended", audit.events[1].Action)
	require.NotContains(t, rec.Body.String(), "echo hello")
}

func TestExecCancelsStreamerWhenClientContextIsDone(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	started := make(chan struct{})
	exec := &fakeExecStreamer{
		run: func(ctx context.Context, req ExecRequest) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		},
	}
	engine := streamTestEngine(memberID, sparkplugin.PluginContext{
		Clusters: fakeStreamClusterFactory{clusterID: clusterID},
		Writer:   allowStreamWriter{allow: true},
	}, nil, exec)
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/clusters/"+clusterID.String()+"/namespaces/default/pods/web-0/exec?command=date", nil).WithContext(ctx)
	req.Header.Set("X-Workspace-Id", wsID.String())
	done := make(chan struct{})

	go func() {
		defer close(done)
		engine.ServeHTTP(httptest.NewRecorder(), req)
	}()
	<-started
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("exec stream did not stop after request context cancellation")
	}
}

func TestSSEWriterEscapesMultilineDataAndSerializesConcurrentWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	writer := newSSEWriter(rec)
	var wg sync.WaitGroup
	for _, event := range []string{"output", "error"} {
		wg.Add(1)
		go func(event string) {
			defer wg.Done()
			require.NoError(t, writer.Event(event, "line 1\nline 2"))
		}(event)
	}
	wg.Wait()

	body := rec.Body.String()
	require.Contains(t, body, "data: \"line 1\\nline 2\"")
	require.NotContains(t, body, "data: line 1\nline 2")
	require.Equal(t, 1, strings.Count(body, "event: output"))
	require.Equal(t, 1, strings.Count(body, "event: error"))
}

func streamTestEngine(memberID kernel.MemberID, ctx sparkplugin.PluginContext, logs LogStreamer, exec ExecStreamer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	engine.Use(httpx.RequireWorkspace())
	handler := NewHandlerWithStreamers(ctx, logs, exec)
	engine.GET("/clusters/:clusterId/namespaces/:namespace/pods/:pod/logs", handler.Logs)
	engine.GET("/clusters/:clusterId/namespaces/:namespace/pods/:pod/exec", handler.Exec)
	return engine
}

type fakeLogStreamer struct {
	body    string
	request LogRequest
}

func (f *fakeLogStreamer) StreamLogs(_ context.Context, _ kubernetes.Interface, req LogRequest) (io.ReadCloser, error) {
	f.request = req
	return io.NopCloser(strings.NewReader(f.body)), nil
}

type fakeExecStreamer struct {
	called  bool
	request ExecRequest
	run     func(context.Context, ExecRequest) error
}

func (f *fakeExecStreamer) StreamExec(ctx context.Context, _ kubernetes.Interface, _ *rest.Config, req ExecRequest) error {
	f.called = true
	f.request = req
	if f.run != nil {
		return f.run(ctx, req)
	}
	return nil
}

type fakeStreamClusterFactory struct {
	clusterID kernel.ClusterInfoID
}

func (f fakeStreamClusterFactory) ForWorkspace(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	return fake.NewSimpleClientset(), nil, &rest.Config{Host: "https://kubernetes.example"}, nil, nil
}

func (f fakeStreamClusterFactory) ListMeta(context.Context, kernel.WorkspaceID) ([]sparkplugin.ClusterMeta, error) {
	return []sparkplugin.ClusterMeta{{ID: f.clusterID.String(), Name: "prod"}}, nil
}

type recordingStreamClusterFactory struct {
	called bool
}

func (f *recordingStreamClusterFactory) ForWorkspace(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	f.called = true
	return fake.NewSimpleClientset(), nil, &rest.Config{Host: "https://kubernetes.example"}, nil, nil
}

func (f *recordingStreamClusterFactory) ListMeta(context.Context, kernel.WorkspaceID) ([]sparkplugin.ClusterMeta, error) {
	return nil, errors.New("not used")
}

type allowStreamWriter struct {
	allow bool
}

func (w allowStreamWriter) CanWrite(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return w.allow, nil
}

type recordingAudit struct {
	events []sparkplugin.AuditEvent
}

func (a *recordingAudit) Record(_ context.Context, event sparkplugin.AuditEvent) error {
	a.events = append(a.events, event)
	return nil
}

func int64Ptr(v int64) *int64 {
	return &v
}

func (r ExecRequest) withoutWriters() ExecRequest {
	r.Stdout = nil
	r.Stderr = nil
	return r
}

package files

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestListExecutesLsWithArgvAndReturnsOutput(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	clusterID := kernel.NewClusterInfoID()
	executor := &recordingExecutor{stdout: "total 0\n-rw-r--r-- 1 root root 0 app.log\n"}
	factory := &recordingClusterFactory{}
	engine := filesEngine(t, sparkplugin.PluginContext{
		Clusters: factory,
		Writer:   allowWriter{allow: true},
	}, executor)

	rec := performFiles(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/namespaces/default/pods/web-0/files?path=/tmp&container=app", wsID, nil, "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "-rw-r--r--")
	require.Equal(t, []kernel.WorkspaceID{wsID}, factory.workspaces)
	require.Equal(t, []string{"ls", "-la", "--", "/tmp"}, executor.calls[0].Command)
	require.Equal(t, "default", executor.calls[0].Namespace)
	require.Equal(t, "web-0", executor.calls[0].Pod)
	require.Equal(t, "app", executor.calls[0].Container)
}

func TestDeleteRequiresWriterBeforeClusterClientOrExec(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	clusterID := kernel.NewClusterInfoID()
	executor := &recordingExecutor{}
	factory := &recordingClusterFactory{}
	engine := filesEngine(t, sparkplugin.PluginContext{
		Clusters: factory,
		Writer:   allowWriter{allow: false},
	}, executor)

	rec := performFiles(engine, http.MethodDelete, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/namespaces/default/pods/web-0/files?path=/tmp/app.log", wsID, nil, "")

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "GOVERNANCE_API_ACCESS_DENIED")
	require.Empty(t, factory.workspaces)
	require.Empty(t, executor.calls)
}

func TestRejectsPathTraversalBeforeExec(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	clusterID := kernel.NewClusterInfoID()
	executor := &recordingExecutor{}
	engine := filesEngine(t, sparkplugin.PluginContext{
		Clusters: &recordingClusterFactory{},
		Writer:   allowWriter{allow: true},
	}, executor)

	rec := performFiles(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/namespaces/default/pods/web-0/files?path=..%2Fsecret", wsID, nil, "")

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "PLUGIN_KUBERNETES_FILE_PATH_INVALID")
	require.Empty(t, executor.calls)
}

func TestUploadStreamsTarWithBaseFilenameOnly(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	clusterID := kernel.NewClusterInfoID()
	executor := &recordingExecutor{}
	engine := filesEngine(t, sparkplugin.PluginContext{
		Clusters: &recordingClusterFactory{},
		Writer:   allowWriter{allow: true},
	}, executor)
	body, contentType := multipartUpload(t, "file", "../evil.txt", "hello")

	rec := performFiles(engine, http.MethodPost, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/namespaces/default/pods/web-0/files?path=/work", wsID, body, contentType)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []string{"tar", "-xmf", "-", "-C", "/work"}, executor.calls[0].Command)
	tr := tar.NewReader(bytes.NewReader(executor.calls[0].Stdin))
	header, err := tr.Next()
	require.NoError(t, err)
	require.Equal(t, "evil.txt", header.Name)
	require.Equal(t, int64(len("hello")), header.Size)
	payload, err := io.ReadAll(tr)
	require.NoError(t, err)
	require.Equal(t, "hello", string(payload))
	_, err = tr.Next()
	require.ErrorIs(t, err, io.EOF)
}

func TestDownloadSetsSafeContentDispositionHeaders(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	clusterID := kernel.NewClusterInfoID()
	executor := &recordingExecutor{stdout: "log bytes"}
	engine := filesEngine(t, sparkplugin.PluginContext{
		Clusters: &recordingClusterFactory{},
		Writer:   allowWriter{allow: true},
	}, executor)

	rec := performFiles(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/namespaces/default/pods/web-0/files/download?path=/var/log/app.log", wsID, nil, "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/octet-stream", rec.Header().Get("Content-Type"))
	disposition := rec.Header().Get("Content-Disposition")
	require.Contains(t, disposition, `attachment; filename="app.log"`)
	require.NotContains(t, disposition, "\r")
	require.NotContains(t, disposition, "\n")
	require.Equal(t, "log bytes", rec.Body.String())
	require.Equal(t, []string{"cat", "--", "/var/log/app.log"}, executor.calls[0].Command)
}

func TestExecFailureDoesNotLeakStderr(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	clusterID := kernel.NewClusterInfoID()
	executor := &recordingExecutor{stderr: "token=secret\ncertificate data", err: errors.New("exec failed")}
	engine := filesEngine(t, sparkplugin.PluginContext{
		Clusters: &recordingClusterFactory{},
		Writer:   allowWriter{allow: true},
	}, executor)

	rec := performFiles(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/namespaces/default/pods/web-0/files?path=/tmp", wsID, nil, "")

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), "PLUGIN_KUBERNETES_FILE_EXEC_FAILED")
	require.NotContains(t, rec.Body.String(), "token=secret")
	require.NotContains(t, rec.Body.String(), "certificate")
}

func filesEngine(t *testing.T, ctx sparkplugin.PluginContext, executor PodExecutor) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	memberID := kernel.NewMemberID()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	handler := NewHandlerWithExecutor(ctx, executor)
	group := engine.Group("/api/v1/plugins/kubernetes")
	group.Use(httpx.RequireWorkspace())
	group.GET("/clusters/:clusterId/namespaces/:namespace/pods/:pod/files", handler.List)
	group.GET("/clusters/:clusterId/namespaces/:namespace/pods/:pod/files/download", handler.Download)
	group.POST("/clusters/:clusterId/namespaces/:namespace/pods/:pod/files", handler.Upload)
	group.DELETE("/clusters/:clusterId/namespaces/:namespace/pods/:pod/files", handler.Delete)
	return engine
}

func performFiles(engine *gin.Engine, method, path string, wsID kernel.WorkspaceID, body io.Reader, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("X-Workspace-Id", wsID.String())
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func multipartUpload(t *testing.T, field, filename, content string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = io.Copy(part, strings.NewReader(content))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return &body, writer.FormDataContentType()
}

type execCall struct {
	Namespace string
	Pod       string
	Container string
	Command   []string
	Stdin     []byte
}

type recordingExecutor struct {
	calls  []execCall
	stdout string
	stderr string
	err    error
}

func (e *recordingExecutor) Exec(_ context.Context, _ kubernetes.Interface, _ *rest.Config, req ExecRequest) error {
	call := execCall{
		Namespace: req.Namespace,
		Pod:       req.Pod,
		Container: req.Container,
		Command:   append([]string(nil), req.Command...),
	}
	if req.Stdin != nil {
		data, _ := io.ReadAll(req.Stdin)
		call.Stdin = data
	}
	e.calls = append(e.calls, call)
	if req.Stdout != nil {
		_, _ = io.WriteString(req.Stdout, e.stdout)
	}
	if req.Stderr != nil {
		_, _ = io.WriteString(req.Stderr, e.stderr)
	}
	return e.err
}

type recordingClusterFactory struct {
	workspaces []kernel.WorkspaceID
}

func (f *recordingClusterFactory) ForWorkspace(_ context.Context, wsID kernel.WorkspaceID, _ kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	f.workspaces = append(f.workspaces, wsID)
	return nil, nil, &rest.Config{Host: "https://kubernetes.example"}, nil, nil
}

func (f *recordingClusterFactory) ListMeta(context.Context, kernel.WorkspaceID) ([]sparkplugin.ClusterMeta, error) {
	return nil, nil
}

type allowWriter struct {
	allow bool
}

func (w allowWriter) CanWrite(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return w.allow, nil
}

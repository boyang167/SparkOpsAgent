package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

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

func TestHandlerAggregatesNamespacedListWithLocalFilters(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "", r.Header.Get("Authorization"))
		require.Equal(t, "", r.Header.Get("Cookie"))
		require.Equal(t, "", r.Header.Get("X-Workspace-Id"))
		require.Equal(t, "app=demo", r.URL.Query().Get("labelSelector"))
		require.Empty(t, r.URL.Query()["page"])
		require.Empty(t, r.URL.Query()["pageSize"])
		require.Empty(t, r.URL.Query()["keyword"])
		require.Empty(t, r.URL.Query()["ns"])

		switch r.URL.Path {
		case "/api/v1/namespaces/dev/pods":
			writeJSON(t, w, http.StatusOK, map[string]any{
				"kind":       "PodList",
				"apiVersion": "v1",
				"metadata":   map[string]any{"resourceVersion": "10"},
				"items": []any{
					map[string]any{"metadata": map[string]any{"name": "web-a", "namespace": "dev"}},
					map[string]any{"metadata": map[string]any{"name": "api-a", "namespace": "dev"}},
				},
			})
		case "/api/v1/namespaces/prod/pods":
			writeJSON(t, w, http.StatusOK, map[string]any{
				"kind":       "PodList",
				"apiVersion": "v1",
				"metadata":   map[string]any{"resourceVersion": "11"},
				"items": []any{
					map[string]any{"metadata": map[string]any{"name": "web-b", "namespace": "prod"}},
					map[string]any{"metadata": map[string]any{"name": "web-c", "namespace": "prod"}},
				},
			})
		default:
			t.Fatalf("unexpected upstream path %q", r.URL.Path)
		}
	}))
	defer upstream.Close()

	engine := proxyTestEngine(memberID, sparkplugin.PluginContext{
		Clusters: proxyClusterFactory{server: upstream.URL, clusterID: clusterID},
		Members:  proxyMembers{allow: true},
		Writer:   proxyWriter{allow: true},
	})

	rec := performProxy(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/proxy/api/v1/pods?page=1&pageSize=2&keyword=web&ns=dev,prod&labelSelector=app%3Ddemo", wsID, nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.JSONEq(t, `{
		"total": 3,
		"items": [
			{"metadata":{"name":"web-a","namespace":"dev"}},
			{"metadata":{"name":"web-b","namespace":"prod"}}
		],
		"page": 1,
		"pageSize": 2
	}`, rec.Body.String())
}

func TestHandlerDoesNotInsertNamespaceForClusterScopedList(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	var seenPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		writeJSON(t, w, http.StatusOK, map[string]any{
			"kind":     "NamespaceList",
			"metadata": map[string]any{"resourceVersion": "12"},
			"items": []any{
				map[string]any{"metadata": map[string]any{"name": "dev"}},
				map[string]any{"metadata": map[string]any{"name": "prod"}},
			},
		})
	}))
	defer upstream.Close()
	engine := proxyTestEngine(memberID, sparkplugin.PluginContext{
		Clusters: proxyClusterFactory{server: upstream.URL, clusterID: clusterID},
		Members:  proxyMembers{allow: true},
		Writer:   proxyWriter{allow: true},
	})

	rec := performProxy(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/proxy/api/v1/namespaces?ns=dev,prod&page=1&pageSize=10", wsID, nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "/api/v1/namespaces", seenPath)
	require.NotContains(t, rec.Body.String(), "/namespaces/dev")
}

func TestHandlerRequiresWritePermissionForMutatingMethods(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	engine := proxyTestEngine(memberID, sparkplugin.PluginContext{
		Clusters: proxyClusterFactory{server: upstream.URL, clusterID: clusterID},
		Members:  proxyMembers{allow: true},
		Writer:   proxyWriter{allow: false},
	})

	rec := performProxy(engine, http.MethodPost, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/proxy/api/v1/namespaces", wsID, strings.NewReader(`{"metadata":{"name":"x"}}`))

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "GOVERNANCE_API_ACCESS_DENIED")
	require.Zero(t, upstreamCalls)
}

func TestHandlerRejectsPathInjectionBeforeClusterLookup(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	factory := &recordingProxyClusterFactory{}
	engine := proxyTestEngine(memberID, sparkplugin.PluginContext{
		Clusters: factory,
		Members:  proxyMembers{allow: true},
		Writer:   proxyWriter{allow: true},
	})

	rec := performProxy(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/proxy/%2e%2e/secrets", wsID, nil)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "PLUGIN_KUBERNETES_PROXY_PATH_INVALID")
	require.Zero(t, factory.calls)
}

func TestHandlerPassesWatchThroughWithoutAggregation(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	var upstreamCalls int
	var seenRawQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		seenRawQuery = r.URL.RawQuery
		require.Equal(t, "/api/v1/pods", r.URL.Path)
		writeJSON(t, w, http.StatusOK, map[string]any{"type": "ADDED"})
	}))
	defer upstream.Close()
	engine := proxyTestEngine(memberID, sparkplugin.PluginContext{
		Clusters: proxyClusterFactory{server: upstream.URL, clusterID: clusterID},
		Members:  proxyMembers{allow: true},
		Writer:   proxyWriter{allow: true},
	})

	rec := performProxy(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/proxy/api/v1/pods?watch=true&page=1&ns=dev", wsID, nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, upstreamCalls)
	require.Contains(t, seenRawQuery, "watch=true")
	require.Contains(t, seenRawQuery, "page=1")
	require.Contains(t, seenRawQuery, "ns=dev")
}

func TestHandlerRedactsSensitiveUpstreamErrors(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"token leaked-message-secret","token":"secret-token","nested":{"client-key":"secret-key","private-key":"secret-private","password":"secret-password","certificate":"secret-cert"}}`))
	}))
	defer upstream.Close()
	engine := proxyTestEngine(memberID, sparkplugin.PluginContext{
		Clusters: proxyClusterFactory{server: upstream.URL, clusterID: clusterID},
		Members:  proxyMembers{allow: true},
		Writer:   proxyWriter{allow: true},
	})

	rec := performProxy(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/proxy/api/v1/pods/bad", wsID, nil)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"token":"redacted"`)
	require.Contains(t, rec.Body.String(), `"client-key":"redacted"`)
	require.NotContains(t, rec.Body.String(), "leaked-message-secret")
	require.NotContains(t, rec.Body.String(), "secret-token")
	require.NotContains(t, rec.Body.String(), "secret-key")
	require.NotContains(t, rec.Body.String(), "secret-private")
	require.NotContains(t, rec.Body.String(), "secret-password")
	require.NotContains(t, rec.Body.String(), "secret-cert")
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(body))
}

func proxyTestEngine(memberID kernel.MemberID, ctx sparkplugin.PluginContext) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	engine.Use(httpx.RequireWorkspace())
	engine.Any("/api/v1/plugins/kubernetes/clusters/:clusterId/proxy/*path", NewHandler(ctx).ServeGIN)
	return engine
}

func performProxy(engine *gin.Engine, method, target string, wsID kernel.WorkspaceID, body *strings.Reader) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == nil {
		reader = strings.NewReader("")
	} else {
		reader = body
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("X-Workspace-Id", wsID.String())
	req.Header.Set("Authorization", "Bearer inbound")
	req.Header.Set("Cookie", "session=inbound")
	req.Header.Set("Connection", "keep-alive")
	rec := newCloseNotifyRecorder()
	engine.ServeHTTP(rec, req)
	return rec.ResponseRecorder
}

type closeNotifyRecorder struct {
	*httptest.ResponseRecorder
	closed chan bool
}

func newCloseNotifyRecorder() *closeNotifyRecorder {
	return &closeNotifyRecorder{ResponseRecorder: httptest.NewRecorder(), closed: make(chan bool, 1)}
}

func (r *closeNotifyRecorder) CloseNotify() <-chan bool {
	return r.closed
}

type proxyClusterFactory struct {
	server    string
	clusterID kernel.ClusterInfoID
}

func (f proxyClusterFactory) ForWorkspace(_ context.Context, _ kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	if clusterID != f.clusterID {
		return nil, nil, nil, nil, kernel.NewDomainError("PLUGIN_CLUSTER_FORBIDDEN", "cluster is not granted to workspace")
	}
	return fake.NewSimpleClientset(), nil, &rest.Config{
		Host: f.server,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}, &sparkplugin.ClusterMeta{ID: clusterID.String(), Server: f.server}, nil
}

func (f proxyClusterFactory) ListMeta(context.Context, kernel.WorkspaceID) ([]sparkplugin.ClusterMeta, error) {
	return nil, nil
}

type recordingProxyClusterFactory struct {
	mu    sync.Mutex
	calls int
}

func (f *recordingProxyClusterFactory) ForWorkspace(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return nil, nil, nil, nil, nil
}

func (f *recordingProxyClusterFactory) ListMeta(context.Context, kernel.WorkspaceID) ([]sparkplugin.ClusterMeta, error) {
	return nil, nil
}

type proxyWriter struct {
	allow bool
}

func (w proxyWriter) CanWrite(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return w.allow, nil
}

type proxyMembers struct {
	allow bool
}

func (m proxyMembers) IsWorkspaceMember(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return m.allow, nil
}

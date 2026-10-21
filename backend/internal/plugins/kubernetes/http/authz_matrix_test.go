package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestAuthzMatrixForProxyRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		body := `{"kind":"Secret","metadata":{"name":"demo"}}`
		if r.URL.Path == "/version" {
			body = `{"gitVersion":"v1.30.0"}`
		}
		_, err := w.Write([]byte(body))
		require.NoError(t, err)
	}))
	defer upstream.Close()

	t.Run("member GET proxy reaches upstream", func(t *testing.T) {
		atomic.StoreInt32(&upstreamCalls, 0)
		engine := authzMatrixEngine(memberID, sparkplugin.PluginContext{
			Clusters: authzClusterFactory{server: upstream.URL, clusterID: clusterID},
			Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
			Writer:   allowWriter{allow: false},
		})
		server := httptest.NewServer(engine)
		defer server.Close()

		status, body := requestAuthzProxy(t, server, http.MethodGet, wsID.String(), clusterID)

		require.Equal(t, http.StatusOK, status, string(body))
		require.Equal(t, int32(1), atomic.LoadInt32(&upstreamCalls))
		require.JSONEq(t, `{"gitVersion":"v1.30.0"}`, string(body))
	})

	t.Run("member POST proxy requires write permission before upstream", func(t *testing.T) {
		atomic.StoreInt32(&upstreamCalls, 0)
		engine := authzMatrixEngine(memberID, sparkplugin.PluginContext{
			Clusters: authzClusterFactory{server: upstream.URL, clusterID: clusterID},
			Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
			Writer:   allowWriter{allow: false},
		})
		server := httptest.NewServer(engine)
		defer server.Close()

		status, body := requestAuthzProxy(t, server, http.MethodPost, wsID.String(), clusterID)

		require.Equal(t, http.StatusForbidden, status, string(body))
		requireEnvelopeError(t, body, "GOVERNANCE_API_ACCESS_DENIED")
		require.Equal(t, int32(0), atomic.LoadInt32(&upstreamCalls))
	})

	t.Run("member Secret GET requires write permission before upstream", func(t *testing.T) {
		atomic.StoreInt32(&upstreamCalls, 0)
		engine := authzMatrixEngine(memberID, sparkplugin.PluginContext{
			Clusters: authzClusterFactory{server: upstream.URL, clusterID: clusterID},
			Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
			Writer:   allowWriter{allow: false},
		})
		server := httptest.NewServer(engine)
		defer server.Close()

		status, body := requestAuthzProxyPath(t, server, http.MethodGet, wsID.String(), clusterID, "/api/v1/namespaces/default/secrets/demo/status")

		require.Equal(t, http.StatusForbidden, status, string(body))
		requireEnvelopeError(t, body, "GOVERNANCE_API_ACCESS_DENIED")
		require.Equal(t, int32(0), atomic.LoadInt32(&upstreamCalls))
	})

	t.Run("admin Secret GET reaches upstream", func(t *testing.T) {
		atomic.StoreInt32(&upstreamCalls, 0)
		engine := authzMatrixEngine(memberID, sparkplugin.PluginContext{
			Clusters: authzClusterFactory{server: upstream.URL, clusterID: clusterID},
			Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
			Writer:   allowWriter{allow: true},
		})
		server := httptest.NewServer(engine)
		defer server.Close()

		status, body := requestAuthzProxyPath(t, server, http.MethodGet, wsID.String(), clusterID, "/api/v1/namespaces/default/secrets/demo")

		require.Equal(t, http.StatusOK, status, string(body))
		require.Equal(t, int32(1), atomic.LoadInt32(&upstreamCalls))
	})

	t.Run("admin POST proxy reaches upstream", func(t *testing.T) {
		atomic.StoreInt32(&upstreamCalls, 0)
		engine := authzMatrixEngine(memberID, sparkplugin.PluginContext{
			Clusters: authzClusterFactory{server: upstream.URL, clusterID: clusterID},
			Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
			Writer:   allowWriter{allow: true},
		})
		server := httptest.NewServer(engine)
		defer server.Close()

		status, body := requestAuthzProxy(t, server, http.MethodPost, wsID.String(), clusterID)

		require.Equal(t, http.StatusOK, status, string(body))
		require.Equal(t, int32(1), atomic.LoadInt32(&upstreamCalls))
	})

	t.Run("workspace header is required", func(t *testing.T) {
		atomic.StoreInt32(&upstreamCalls, 0)
		engine := authzMatrixEngine(memberID, sparkplugin.PluginContext{
			Clusters: authzClusterFactory{server: upstream.URL, clusterID: clusterID},
			Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
			Writer:   allowWriter{allow: true},
		})
		server := httptest.NewServer(engine)
		defer server.Close()

		status, body := requestAuthzProxy(t, server, http.MethodGet, "", clusterID)

		require.Equal(t, http.StatusBadRequest, status, string(body))
		requireEnvelopeError(t, body, "TENANT_WORKSPACE_REQUIRED")
		require.Equal(t, int32(0), atomic.LoadInt32(&upstreamCalls))
	})

	t.Run("non member is rejected before upstream", func(t *testing.T) {
		atomic.StoreInt32(&upstreamCalls, 0)
		engine := authzMatrixEngine(memberID, sparkplugin.PluginContext{
			Clusters: authzClusterFactory{server: upstream.URL, clusterID: clusterID},
			Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{}},
			Writer:   allowWriter{allow: true},
		})
		server := httptest.NewServer(engine)
		defer server.Close()

		status, body := requestAuthzProxy(t, server, http.MethodGet, wsID.String(), clusterID)

		require.Equal(t, http.StatusForbidden, status, string(body))
		requireEnvelopeError(t, body, "TENANT_NOT_WORKSPACE_MEMBER")
		require.Equal(t, int32(0), atomic.LoadInt32(&upstreamCalls))
	})
}

func authzMatrixEngine(memberID kernel.MemberID, ctx sparkplugin.PluginContext) *gin.Engine {
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	RegisterRoutes(engine, ctx)
	return engine
}

func requestAuthzProxy(t *testing.T, server *httptest.Server, method, workspaceID string, clusterID kernel.ClusterInfoID) (int, []byte) {
	return requestAuthzProxyPath(t, server, method, workspaceID, clusterID, "/version")
}

func requestAuthzProxyPath(t *testing.T, server *httptest.Server, method, workspaceID string, clusterID kernel.ClusterInfoID, path string) (int, []byte) {
	t.Helper()

	req, err := http.NewRequest(method, server.URL+"/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/proxy"+path, nil)
	require.NoError(t, err)
	if workspaceID != "" {
		req.Header.Set("X-Workspace-Id", workspaceID)
	}
	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

func requireEnvelopeError(t *testing.T, body []byte, errorCode string) {
	t.Helper()

	var env struct {
		Code int               `json:"code"`
		Data map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &env))
	require.Equal(t, 1, env.Code)
	require.Equal(t, errorCode, env.Data["error_code"])
}

type authzClusterFactory struct {
	server    string
	clusterID kernel.ClusterInfoID
}

func (f authzClusterFactory) ForWorkspace(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	meta := sparkplugin.ClusterMeta{ID: f.clusterID.String(), Name: "prod", Server: f.server}
	return nil, nil, &rest.Config{Host: f.server}, &meta, nil
}

func (f authzClusterFactory) ListMeta(context.Context, kernel.WorkspaceID) ([]sparkplugin.ClusterMeta, error) {
	return []sparkplugin.ClusterMeta{{ID: f.clusterID.String(), Name: "prod", Server: f.server}}, nil
}

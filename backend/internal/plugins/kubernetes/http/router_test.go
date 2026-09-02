package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestRegisterRoutesPanicsWhenMountedTwiceOnSameEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterRoutes(engine)

	require.Panics(t, func() {
		RegisterRoutes(engine)
	})
}

func TestRegisterRoutesMountsProxyForHeadRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := kernel.NewMemberID()
	wsID := kernel.NewWorkspaceID()
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	RegisterRoutes(engine, sparkplugin.PluginContext{
		Members: allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	req := httptest.NewRequest(http.MethodHead, "/api/v1/plugins/kubernetes/clusters/"+kernel.NewClusterInfoID().String()+"/proxy/version", nil)
	req.Header.Set("X-Workspace-Id", wsID.String())
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestRegisterRoutesMountsPodFilesRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := kernel.NewMemberID()
	wsID := kernel.NewWorkspaceID()
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	RegisterRoutes(engine, sparkplugin.PluginContext{
		Members: allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	for _, path := range []string{
		"/api/v1/plugins/kubernetes/clusters/" + kernel.NewClusterInfoID().String() + "/namespaces/default/pods/web-0/files?path=/tmp",
		"/api/v1/plugins/kubernetes/clusters/" + kernel.NewClusterInfoID().String() + "/namespaces/default/pods/web-0/files/download?path=/tmp/app.log",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Workspace-Id", wsID.String())
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	}
}

func TestRegisterRoutesMountsPodStreamRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := kernel.NewMemberID()
	wsID := kernel.NewWorkspaceID()
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	RegisterRoutes(engine, sparkplugin.PluginContext{
		Members: allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	for _, path := range []string{
		"/api/v1/plugins/kubernetes/clusters/" + kernel.NewClusterInfoID().String() + "/namespaces/default/pods/web-0/logs",
		"/api/v1/plugins/kubernetes/clusters/" + kernel.NewClusterInfoID().String() + "/namespaces/default/pods/web-0/exec?command=date",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Workspace-Id", wsID.String())
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	}
}

func TestRegisterRoutesMountsPodTerminalRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := kernel.NewMemberID()
	wsID := kernel.NewWorkspaceID()
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	RegisterRoutes(engine, sparkplugin.PluginContext{
		Members: allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/plugins/kubernetes/clusters/"+kernel.NewClusterInfoID().String()+"/namespaces/default/pods/web-0/terminal-ticket",
		nil,
	)
	req.Header.Set("X-Workspace-Id", wsID.String())
	engine.ServeHTTP(rec, req)
	require.NotEqual(t, http.StatusNotFound, rec.Code)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/ws/plugins/kubernetes/terminal?ticket=missing", nil)
	engine.ServeHTTP(rec, req)
	require.NotEqual(t, http.StatusNotFound, rec.Code)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRegisterRoutesMountsRecordRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := kernel.NewMemberID()
	wsID := kernel.NewWorkspaceID()
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	RegisterRoutes(engine, sparkplugin.PluginContext{
		Members: allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	for _, path := range []string{
		"/api/v1/plugins/kubernetes/records",
		"/api/v1/plugins/kubernetes/records/" + kernel.NewPluginInstallID().String(),
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Workspace-Id", wsID.String())
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		require.NotEqual(t, http.StatusNotFound, rec.Code)
		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	}
}

func TestRegisterRoutesMountsTTYRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := kernel.NewMemberID()
	wsID := kernel.NewWorkspaceID()
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	RegisterRoutes(engine, sparkplugin.PluginContext{
		Members: allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	for _, path := range []string{
		"/api/v1/plugins/kubernetes/clusters/" + kernel.NewClusterInfoID().String() + "/cloudtty",
		"/api/v1/plugins/kubernetes/clusters/" + kernel.NewClusterInfoID().String() + "/nodetty",
	} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("X-Workspace-Id", wsID.String())
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		require.NotEqual(t, http.StatusNotFound, rec.Code)
		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	}
}

func TestRegisterRoutesMountsTask15Routes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := kernel.NewMemberID()
	wsID := kernel.NewWorkspaceID()
	clusterID := kernel.NewClusterInfoID()
	diagnosisID := kernel.NewPluginInstallID()
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	RegisterRoutes(engine, sparkplugin.PluginContext{
		Members: allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/plugins/kubernetes/clusters/" + clusterID.String() + "/diagnosis"},
		{http.MethodPost, "/api/v1/plugins/kubernetes/clusters/" + clusterID.String() + "/diagnosis/install"},
		{http.MethodDelete, "/api/v1/plugins/kubernetes/clusters/" + clusterID.String() + "/diagnosis/uninstall"},
		{http.MethodPost, "/api/v1/plugins/kubernetes/clusters/" + clusterID.String() + "/diagnosis/by-pod"},
		{http.MethodPost, "/api/v1/plugins/kubernetes/clusters/" + clusterID.String() + "/diagnosis/jps"},
		{http.MethodPost, "/api/v1/plugins/kubernetes/clusters/" + clusterID.String() + "/diagnosis/proxy"},
		{http.MethodGet, "/api/v1/plugins/kubernetes/diagnosis/" + diagnosisID.String() + "/ws"},
		{http.MethodDelete, "/api/v1/plugins/kubernetes/diagnosis/" + diagnosisID.String()},
		{http.MethodGet, "/api/v1/plugins/kubernetes/clusters/" + clusterID.String() + "/velaux/ui"},
		{http.MethodPost, "/api/v1/plugins/kubernetes/clusters/" + clusterID.String() + "/chat/query"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("X-Workspace-Id", wsID.String())
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		require.NotEqual(t, http.StatusNotFound, rec.Code, tc.path)
		require.Equal(t, http.StatusServiceUnavailable, rec.Code, tc.path)
	}
}

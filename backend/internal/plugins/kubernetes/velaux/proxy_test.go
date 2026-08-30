package velaux

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestProxyForwardsSafePathAndAllowlistedHeaders(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	var gotPath, gotQuery, gotAuth, gotAccept, gotCookie, gotWorkspace string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotCookie = r.Header.Get("Cookie")
		gotWorkspace = r.Header.Get("X-Workspace-Id")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(upstream.Close)
	db := openVelauxDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{
		ClusterID:      clusterID.String(),
		VelauxURL:      upstream.URL + "/base",
		VelauxUser:     "vela",
		VelauxPassword: "secret",
	}).Error)
	engine := velauxEngine(memberID, NewHandler(sparkplugin.PluginContext{
		DB:       db,
		Clusters: velauxClusterFactory{clusterID: clusterID},
		Writer:   allowWriter{allow: true},
	}))

	req := httptest.NewRequest(http.MethodGet, "/clusters/"+clusterID.String()+"/velaux/ui/api/apps?limit=1", nil)
	req.Header.Set("X-Workspace-Id", wsID.String())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cookie", "session=leak")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "/base/ui/api/apps", gotPath)
	require.Equal(t, "limit=1", gotQuery)
	require.Equal(t, "Basic dmVsYTpzZWNyZXQ=", gotAuth)
	require.Equal(t, "application/json", gotAccept)
	require.Empty(t, gotCookie)
	require.Empty(t, gotWorkspace)
}

func TestProxyRejectsUnsafePathAndMissingURLWithoutCallingUpstream(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)
	db := openVelauxDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{ClusterID: clusterID.String(), VelauxURL: upstream.URL}).Error)
	engine := velauxEngine(memberID, NewHandler(sparkplugin.PluginContext{
		DB:       db,
		Clusters: velauxClusterFactory{clusterID: clusterID},
		Writer:   allowWriter{allow: true},
	}))

	for _, path := range []string{
		"/clusters/" + clusterID.String() + "/velaux/../secret",
		"/clusters/" + clusterID.String() + "/velaux/http://evil.test/path",
		"/clusters/" + clusterID.String() + "/velaux//evil.test/path",
		"/clusters/" + clusterID.String() + "/velaux/%252e%252e/secret",
		"/clusters/" + clusterID.String() + "/velaux/%255cwindows",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Workspace-Id", wsID.String())
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	require.Zero(t, calls)

	emptyDB := openVelauxDB(t)
	emptyEngine := velauxEngine(memberID, NewHandler(sparkplugin.PluginContext{
		DB:       emptyDB,
		Clusters: velauxClusterFactory{clusterID: clusterID},
		Writer:   allowWriter{allow: true},
	}))
	req := httptest.NewRequest(http.MethodGet, "/clusters/"+clusterID.String()+"/velaux/ui", nil)
	req.Header.Set("X-Workspace-Id", wsID.String())
	rec := httptest.NewRecorder()
	emptyEngine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "PLUGIN_VELAUX_NOT_CONFIGURED")
}

func TestProxyRequiresWriterForMutatingMethodsBeforeUpstream(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	called := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)
	db := openVelauxDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{ClusterID: clusterID.String(), VelauxURL: upstream.URL}).Error)
	engine := velauxEngine(memberID, NewHandler(sparkplugin.PluginContext{
		DB:       db,
		Clusters: velauxClusterFactory{clusterID: clusterID},
		Writer:   allowWriter{allow: false},
	}))

	rec := performVelaux(engine, http.MethodPost, "/clusters/"+clusterID.String()+"/velaux/api/apps", wsID, []byte(`{}`))

	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	require.False(t, called)
}

func TestProxyCapsResponseBody(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "16777217")
		_, _ = io.Copy(w, io.LimitReader(bytes.NewReader(bytes.Repeat([]byte("x"), int(MaxResponseBodyBytes+1))), MaxResponseBodyBytes+1))
	}))
	t.Cleanup(upstream.Close)
	db := openVelauxDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{ClusterID: clusterID.String(), VelauxURL: upstream.URL}).Error)
	engine := velauxEngine(memberID, NewHandler(sparkplugin.PluginContext{
		DB:       db,
		Clusters: velauxClusterFactory{clusterID: clusterID},
		Writer:   allowWriter{allow: true},
	}))

	rec := performVelaux(engine, http.MethodGet, "/clusters/"+clusterID.String()+"/velaux/api/apps", wsID, nil)

	require.Equal(t, http.StatusBadGateway, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "PLUGIN_VELAUX_UPSTREAM_UNAVAILABLE")
}

func TestProxyCapsChunkedJSONResponseBody(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		chunk := bytes.Repeat([]byte("x"), 32<<10)
		for written := int64(0); written <= MaxResponseBodyBytes; written += int64(len(chunk)) {
			_, _ = w.Write(chunk)
			w.(http.Flusher).Flush()
		}
	}))
	t.Cleanup(upstream.Close)
	db := openVelauxDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{ClusterID: clusterID.String(), VelauxURL: upstream.URL}).Error)
	engine := velauxEngine(memberID, NewHandler(sparkplugin.PluginContext{
		DB:       db,
		Clusters: velauxClusterFactory{clusterID: clusterID},
		Writer:   allowWriter{allow: true},
	}))

	rec := performVelaux(engine, http.MethodGet, "/clusters/"+clusterID.String()+"/velaux/api/apps", wsID, nil)

	require.Equal(t, http.StatusBadGateway, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "PLUGIN_VELAUX_UPSTREAM_UNAVAILABLE")
}

func TestProxyAllowsSmallChunkedJSONResponse(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":`))
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte(`true}`))
	}))
	t.Cleanup(upstream.Close)
	db := openVelauxDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{ClusterID: clusterID.String(), VelauxURL: upstream.URL}).Error)
	engine := velauxEngine(memberID, NewHandler(sparkplugin.PluginContext{
		DB:       db,
		Clusters: velauxClusterFactory{clusterID: clusterID},
		Writer:   allowWriter{allow: true},
	}))

	rec := performVelaux(engine, http.MethodGet, "/clusters/"+clusterID.String()+"/velaux/api/apps", wsID, nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.JSONEq(t, `{"ok":true}`, rec.Body.String())
}

func TestProxyStreamsEventStreamBeforeUpstreamEnds(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	firstChunkWritten := make(chan struct{}, 1)
	releaseUpstream := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(releaseUpstream)
		}
	}()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		_, _ = w.Write([]byte("data: first\n\n"))
		w.(http.Flusher).Flush()
		firstChunkWritten <- struct{}{}
		<-releaseUpstream
		_, _ = w.Write([]byte("data: second\n\n"))
		w.(http.Flusher).Flush()
	}))
	t.Cleanup(upstream.Close)
	db := openVelauxDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{ClusterID: clusterID.String(), VelauxURL: upstream.URL}).Error)
	engine := velauxEngine(memberID, NewHandler(sparkplugin.PluginContext{
		DB:       db,
		Clusters: velauxClusterFactory{clusterID: clusterID},
		Writer:   allowWriter{allow: true},
	}))
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

	respCh := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/clusters/"+clusterID.String()+"/velaux/events", nil)
		require.NoError(t, err)
		req.Header.Set("X-Workspace-Id", wsID.String())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}()

	<-firstChunkWritten
	var resp *http.Response
	select {
	case resp = <-respCh:
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("first stream chunk was not visible before upstream completed")
	}
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "data: first\n", line)
	close(releaseUpstream)
	released = true
	rest, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(rest), "data: second")
}

func TestProxyStreamStopsWhenClientCancels(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	upstreamStarted := make(chan struct{}, 1)
	upstreamCanceled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: first\n\n"))
		w.(http.Flusher).Flush()
		upstreamStarted <- struct{}{}
		<-r.Context().Done()
		upstreamCanceled <- struct{}{}
	}))
	t.Cleanup(upstream.Close)
	db := openVelauxDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{ClusterID: clusterID.String(), VelauxURL: upstream.URL}).Error)
	engine := velauxEngine(memberID, NewHandler(sparkplugin.PluginContext{
		DB:       db,
		Clusters: velauxClusterFactory{clusterID: clusterID},
		Writer:   allowWriter{allow: true},
	}))
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/clusters/"+clusterID.String()+"/velaux/events", strings.NewReader(""))
	require.NoError(t, err)
	req.Header.Set("X-Workspace-Id", wsID.String())
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	<-upstreamStarted
	cancel()

	select {
	case <-upstreamCanceled:
	case <-time.After(time.Second):
		t.Fatal("upstream did not observe downstream cancellation")
	}
}

func openVelauxDB(t *testing.T) *gorm.DB {
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

func velauxEngine(memberID kernel.MemberID, handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	engine.Use(httpx.RequireWorkspace())
	engine.Any("/clusters/:clusterId/velaux/*path", handler.ServeGIN)
	return engine
}

func performVelaux(engine *gin.Engine, method, path string, wsID kernel.WorkspaceID, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("X-Workspace-Id", wsID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

type velauxClusterFactory struct {
	clusterID kernel.ClusterInfoID
}

func (f velauxClusterFactory) ForWorkspace(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	return nil, nil, nil, &sparkplugin.ClusterMeta{ID: f.clusterID.String(), Name: "prod"}, nil
}

func (f velauxClusterFactory) ListMeta(context.Context, kernel.WorkspaceID) ([]sparkplugin.ClusterMeta, error) {
	return []sparkplugin.ClusterMeta{{ID: f.clusterID.String(), Name: "prod"}}, nil
}

type allowWriter struct {
	allow bool
}

func (w allowWriter) CanWrite(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return w.allow, nil
}

var _ = json.Valid

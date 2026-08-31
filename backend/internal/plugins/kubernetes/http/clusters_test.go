package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestListClustersReturnsAuthorizedMetadataWithMaskedSettings(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	forbiddenID := kernel.NewClusterInfoID()
	db := openK8sHTTPDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{
		WorkspaceID:        wsID.String(),
		ClusterID:          clusterID.String(),
		PrometheusURL:      "https://prom.example",
		PrometheusUser:     "prom-user",
		PrometheusPassword: "prom-secret",
		VelauxURL:          "https://velaux.example",
		VelauxUser:         "velaux-user",
		VelauxPassword:     "",
		LLMBaseURL:         "https://llm.example/v1",
		LLMAPIKey:          "llm-secret",
		LLMModel:           "qwen",
	}).Error)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{
		WorkspaceID:        kernel.NewWorkspaceID().String(),
		ClusterID:          forbiddenID.String(),
		PrometheusURL:      "https://forbidden.example",
		PrometheusPassword: "do-not-leak",
		LLMAPIKey:          "do-not-leak",
	}).Error)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB: db,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{
			ID:     clusterID.String(),
			Name:   "prod",
			Server: "https://kubernetes.example",
		}}},
		Members: allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
		Writer:  allowWriter{allow: true},
	})

	rec := performJSON(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters", wsID, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	var env httpx.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	body, err := json.Marshal(env.Data)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"items":[{
			"id":"`+clusterID.String()+`",
			"name":"prod",
			"server":"https://kubernetes.example",
			"prometheus_url":"https://prom.example",
			"prometheus_user":"prom-user",
			"prometheus_password":"***",
			"velaux_url":"https://velaux.example",
			"velaux_user":"velaux-user",
			"velaux_password":"",
			"llm_base_url":"https://llm.example/v1",
			"llm_api_key":"***",
			"llm_model":"qwen"
		}]
	}`, string(body))
	require.NotContains(t, rec.Body.String(), forbiddenID.String())
	require.NotContains(t, rec.Body.String(), "do-not-leak")
	require.NotContains(t, rec.Body.String(), "prom-secret")
	require.NotContains(t, rec.Body.String(), "llm-secret")
}

func TestPutClusterSettingsRequiresGovernanceAdmin(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openK8sHTTPDB(t)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "prod"}}},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
		Writer:   allowWriter{allow: false},
	})

	rec := performJSON(engine, http.MethodPut, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/settings", wsID, map[string]string{
		"prometheus_url": "https://prom.example",
	})

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "GOVERNANCE_API_ACCESS_DENIED")
	var count int64
	require.NoError(t, db.Model(&k8sschema.ClusterSettings{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestGetKubeconfigReturnsYAMLForGovernanceAdmin(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB: openK8sHTTPDB(t),
		Clusters: fakeClusterFactory{
			items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "dev"}},
			rest: &rest.Config{
				Host:        "https://k8s.example:6443",
				BearerToken: "live-token",
				TLSClientConfig: rest.TLSClientConfig{
					Insecure: true,
				},
			},
		},
		Members: allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
		Writer:  allowWriter{allow: true},
	})

	rec := performJSON(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/kubeconfig", wsID, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	var env httpx.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	body, err := json.Marshal(env.Data)
	require.NoError(t, err)
	var view kubeconfigView
	require.NoError(t, json.Unmarshal(body, &view))
	require.Contains(t, view.Kubeconfig, "https://k8s.example:6443")
	require.Contains(t, view.Kubeconfig, "live-token")
}

func TestGetKubeconfigRequiresGovernanceAdmin(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       openK8sHTTPDB(t),
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "dev"}}},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
		Writer:   allowWriter{allow: false},
	})

	rec := performJSON(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/kubeconfig", wsID, nil)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "GOVERNANCE_API_ACCESS_DENIED")
}

func TestPutClusterSettingsRejectsUngrantCluster(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openK8sHTTPDB(t)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: nil},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
		Writer:   allowWriter{allow: true},
	})

	rec := performJSON(engine, http.MethodPut, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/settings", wsID, map[string]string{
		"prometheus_url": "https://prom.example",
	})

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "PLUGIN_CLUSTER_FORBIDDEN")
	var count int64
	require.NoError(t, db.Model(&k8sschema.ClusterSettings{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestPutClusterSettingsPreservesMaskedSecretsAndAllowsClearing(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openK8sHTTPDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{
		WorkspaceID:        wsID.String(),
		ClusterID:          clusterID.String(),
		PrometheusPassword: "existing-prom",
		VelauxPassword:     "existing-velaux",
		LLMAPIKey:          "existing-key",
	}).Error)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "prod"}}},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
		Writer:   allowWriter{allow: true},
	})

	rec := performJSON(engine, http.MethodPut, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/settings", wsID, map[string]string{
		"prometheus_url":      "https://prometheus.example",
		"prometheus_user":     "prom-user",
		"prometheus_password": "***",
		"velaux_url":          "",
		"velaux_user":         "velaux-user",
		"velaux_password":     "",
		"llm_base_url":        "https://llm.example/v1",
		"llm_api_key":         "***",
		"llm_model":           "qwen-plus",
	})

	require.Equal(t, http.StatusOK, rec.Code)
	var stored k8sschema.ClusterSettings
	require.NoError(t, db.First(&stored, "workspace_id = ? AND cluster_id = ?", wsID.String(), clusterID.String()).Error)
	require.Equal(t, "https://prometheus.example", stored.PrometheusURL)
	require.Equal(t, "prom-user", stored.PrometheusUser)
	require.Equal(t, "existing-prom", stored.PrometheusPassword)
	require.Equal(t, "", stored.VelauxURL)
	require.Equal(t, "velaux-user", stored.VelauxUser)
	require.Equal(t, "", stored.VelauxPassword)
	require.Equal(t, "https://llm.example/v1", stored.LLMBaseURL)
	require.Equal(t, "existing-key", stored.LLMAPIKey)
	require.Equal(t, "qwen-plus", stored.LLMModel)
	require.NotContains(t, rec.Body.String(), "existing-prom")
	require.NotContains(t, rec.Body.String(), "existing-key")
	require.Contains(t, rec.Body.String(), `"prometheus_password":"***"`)
	require.Contains(t, rec.Body.String(), `"llm_api_key":"***"`)
}

func TestClusterSettingsAreIsolatedByWorkspaceForSameCluster(t *testing.T) {
	wsA := kernel.NewWorkspaceID()
	wsB := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openK8sHTTPDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{
		WorkspaceID: wsA.String(), ClusterID: clusterID.String(),
		PrometheusURL: "https://a.example", PrometheusPassword: "secret-a",
	}).Error)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{
		WorkspaceID: wsB.String(), ClusterID: clusterID.String(),
		PrometheusURL: "https://b.example", PrometheusPassword: "secret-b",
	}).Error)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "shared"}}},
		Members: allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{
			wsA: {memberID: true}, wsB: {memberID: true},
		}},
		Writer: allowWriter{allow: true},
	})

	recA := performJSON(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters", wsA, nil)
	recB := performJSON(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters", wsB, nil)
	require.Contains(t, recA.Body.String(), "https://a.example")
	require.NotContains(t, recA.Body.String(), "https://b.example")
	require.Contains(t, recB.Body.String(), "https://b.example")
	require.NotContains(t, recB.Body.String(), "https://a.example")

	update := performJSON(engine, http.MethodPut, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/settings", wsA, map[string]string{
		"prometheus_url": "https://a2.example", "prometheus_password": "***",
	})
	require.Equal(t, http.StatusOK, update.Code, update.Body.String())
	var rowA, rowB k8sschema.ClusterSettings
	require.NoError(t, db.First(&rowA, "workspace_id = ? AND cluster_id = ?", wsA.String(), clusterID.String()).Error)
	require.NoError(t, db.First(&rowB, "workspace_id = ? AND cluster_id = ?", wsB.String(), clusterID.String()).Error)
	require.Equal(t, "secret-a", rowA.PrometheusPassword)
	require.Equal(t, "secret-b", rowB.PrometheusPassword)
	require.Equal(t, "https://b.example", rowB.PrometheusURL)
}

func TestPutClusterSettingsValidatesURLFields(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openK8sHTTPDB(t)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "prod"}}},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
		Writer:   allowWriter{allow: true},
	})

	for _, payload := range []map[string]string{
		{"prometheus_url": "ftp://prometheus.example"},
		{"velaux_url": "http://[::1"},
		{"llm_base_url": "//llm.example"},
		{"prometheus_url": "https://user:pass@prometheus.example"},
	} {
		rec := performJSON(engine, http.MethodPut, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/settings", wsID, payload)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "PLUGIN_CLUSTER_SETTINGS_INVALID_URL")
		require.NotContains(t, rec.Body.String(), "user:pass")
		require.NotContains(t, rec.Body.String(), "prometheus.example")
	}
}

func TestPutClusterSettingsRejectsUnsafeAndResolvedPrivateEndpoints(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	policy, err := sparkplugin.NewEndpointPolicy(nil, settingsResolver{
		"private.example": {{IP: net.ParseIP("192.168.1.20")}},
	})
	require.NoError(t, err)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB: openK8sHTTPDB(t), Endpoints: policy,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String()}}},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
		Writer:   allowWriter{allow: true},
	})
	for _, endpoint := range []string{
		"http://127.0.0.1:9090", "http://169.254.169.254", "http://10.0.0.1", "http://private.example",
	} {
		rec := performJSON(engine, http.MethodPut, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/settings", wsID, map[string]string{
			"prometheus_url": endpoint,
		})
		require.Equal(t, http.StatusBadRequest, rec.Code, endpoint)
		require.Contains(t, rec.Body.String(), "PLUGIN_CLUSTER_SETTINGS_INVALID_URL")
		require.NotContains(t, rec.Body.String(), endpoint)
	}
}

func TestPutClusterSettingsAllowsExplicitEndpointAllowlist(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer upstream.Close()
	policy, err := sparkplugin.NewEndpointPolicy([]string{"127.0.0.1"}, nil)
	require.NoError(t, err)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB: openK8sHTTPDB(t), Endpoints: policy,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String()}}},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
		Writer:   allowWriter{allow: true},
	})
	rec := performJSON(engine, http.MethodPut, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/settings", wsID, map[string]string{
		"prometheus_url": upstream.URL,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestClusterRoutesReturnServiceUnavailableWithoutPluginContext(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	engine := testEngine(memberID, sparkplugin.PluginContext{})

	getRec := performJSON(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters", wsID, nil)
	require.Equal(t, http.StatusServiceUnavailable, getRec.Code)

	putRec := performJSON(engine, http.MethodPut, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/settings", wsID, map[string]string{
		"prometheus_url": "https://prom.example",
	})
	require.Equal(t, http.StatusServiceUnavailable, putRec.Code)
}

func TestClusterRoutesRequireWorkspaceMembershipFromProductionChain(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	otherWSID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openK8sHTTPDB(t)
	factory := &recordingClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "prod"}}}
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: factory,
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
		Writer:   allowWriter{allow: true},
	})

	okRec := performJSON(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters", wsID, nil)
	require.Equal(t, http.StatusOK, okRec.Code)
	require.Equal(t, []kernel.WorkspaceID{wsID}, factory.seenWorkspaces)

	forbiddenRec := performJSON(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters", otherWSID, nil)
	require.Equal(t, http.StatusForbidden, forbiddenRec.Code)
	require.Contains(t, forbiddenRec.Body.String(), "TENANT_NOT_WORKSPACE_MEMBER")
	require.Equal(t, []kernel.WorkspaceID{wsID}, factory.seenWorkspaces)
}

func TestClusterRoutesReturnServiceUnavailableWithoutMemberAuthorizer(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openK8sHTTPDB(t)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "prod"}}},
		Writer:   allowWriter{allow: true},
	})

	rec := performJSON(engine, http.MethodGet, "/api/v1/plugins/kubernetes/clusters", wsID, nil)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func openK8sHTTPDB(t *testing.T) *gorm.DB {
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

func testEngine(memberID kernel.MemberID, ctx sparkplugin.PluginContext) *gin.Engine {
	gin.SetMode(gin.TestMode)
	if ctx.Endpoints == nil {
		policy, _ := sparkplugin.NewEndpointPolicy([]string{
			"127.0.0.1", "prom.example", "prometheus.example", "a2.example", "velaux.example", "llm.example",
		}, settingsResolver{
			"prom.example":       {{IP: net.ParseIP("203.0.113.10")}},
			"prometheus.example": {{IP: net.ParseIP("203.0.113.11")}},
			"a2.example":         {{IP: net.ParseIP("203.0.113.12")}},
			"velaux.example":     {{IP: net.ParseIP("203.0.113.13")}},
			"llm.example":        {{IP: net.ParseIP("203.0.113.14")}},
		})
		ctx.Endpoints = policy
	}
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	RegisterRoutes(engine, ctx)
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

type fakeClusterFactory struct {
	items  []sparkplugin.ClusterMeta
	rest   *rest.Config
	client kubernetes.Interface
	forErr error
}

func (f fakeClusterFactory) ForWorkspace(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	if f.forErr != nil {
		return nil, nil, nil, nil, f.forErr
	}
	var meta *sparkplugin.ClusterMeta
	if len(f.items) > 0 {
		item := f.items[0]
		meta = &item
	}
	return f.client, nil, f.rest, meta, nil
}

func (f fakeClusterFactory) ListMeta(context.Context, kernel.WorkspaceID) ([]sparkplugin.ClusterMeta, error) {
	return f.items, nil
}

type allowWriter struct {
	allow bool
}

func (w allowWriter) CanWrite(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return w.allow, nil
}

type allowMembers struct {
	grants map[kernel.WorkspaceID]map[kernel.MemberID]bool
}

func (m allowMembers) IsWorkspaceMember(_ context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) (bool, error) {
	return m.grants[wsID][memberID], nil
}

type recordingClusterFactory struct {
	items          []sparkplugin.ClusterMeta
	seenWorkspaces []kernel.WorkspaceID
}

type settingsResolver map[string][]net.IPAddr

func (r settingsResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	return r[host], nil
}

func (f *recordingClusterFactory) ForWorkspace(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	return nil, nil, nil, nil, nil
}

func (f *recordingClusterFactory) ListMeta(_ context.Context, wsID kernel.WorkspaceID) ([]sparkplugin.ClusterMeta, error) {
	f.seenWorkspaces = append(f.seenWorkspaces, wsID)
	return f.items, nil
}

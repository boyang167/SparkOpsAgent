package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestPostMetricsProxiesNamedTemplateToPrometheus(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	var seen bool
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = true
		require.Equal(t, "/api/v1/query_range", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		user, password, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "prom-user", user)
		require.Equal(t, "prom-secret", password)
		require.Equal(t, `sum(rate(container_cpu_usage_seconds_total{image!="",pod="api-0",namespace="default"}[1m])) by (pod,namespace)`, r.URL.Query().Get("query"))
		require.Equal(t, "1700003300", r.URL.Query().Get("start"))
		require.Equal(t, "1700003600", r.URL.Query().Get("end"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"pod":"api-0"},"values":[[1700003600,"0.25"]]}]}}`))
	}))
	t.Cleanup(prometheus.Close)
	db := openK8sHTTPDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{
		WorkspaceID:        wsID.String(),
		ClusterID:          clusterID.String(),
		PrometheusURL:      prometheus.URL,
		PrometheusUser:     "prom-user",
		PrometheusPassword: "prom-secret",
	}).Error)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "prod"}}},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	rec := performJSON(engine, http.MethodPost, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/metrics", wsID, map[string]string{
		"template":  "pod_cpu",
		"range":     "5m",
		"namespace": "default",
		"pod":       "api-0",
		"time":      "1700003600",
	})

	require.True(t, seen)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"resultType":"matrix"`)
	require.Contains(t, rec.Body.String(), `"pod":"api-0"`)
	require.NotContains(t, rec.Body.String(), "prom-secret")
}

func TestPostMetricsRejectsArbitraryPromQLField(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       openK8sHTTPDB(t),
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "prod"}}},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	rec := performJSON(engine, http.MethodPost, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/metrics", wsID, map[string]string{
		"query": "up",
	})

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "PLUGIN_METRICS_INVALID_REQUEST")
	require.NotContains(t, rec.Body.String(), "up")
}

func TestPostMetricsRequiresAuthorizedClusterBeforeSettingsLookup(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("prometheus must not be called for unauthorized clusters")
	}))
	t.Cleanup(prometheus.Close)
	db := openK8sHTTPDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{
		WorkspaceID:        wsID.String(),
		ClusterID:          clusterID.String(),
		PrometheusURL:      prometheus.URL,
		PrometheusPassword: "do-not-leak",
	}).Error)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: nil},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	rec := performJSON(engine, http.MethodPost, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/metrics", wsID, map[string]string{
		"template": "node_cpu",
		"range":    "5m",
		"node":     "node-1",
	})

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "PLUGIN_CLUSTER_FORBIDDEN")
	require.NotContains(t, rec.Body.String(), prometheus.URL)
	require.NotContains(t, rec.Body.String(), "do-not-leak")
}

func TestPostMetricsReturnsNotConfiguredWithoutPrometheusURL(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       openK8sHTTPDB(t),
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "prod"}}},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	rec := performJSON(engine, http.MethodPost, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/metrics", wsID, map[string]string{
		"template": "node_memory",
		"range":    "15m",
		"node":     "node-1",
	})

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "PLUGIN_PROMETHEUS_NOT_CONFIGURED")
}

func TestPostMetricsHidesUpstreamFailures(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `prom-secret http://internal-prometheus.example`, http.StatusUnauthorized)
	}))
	t.Cleanup(prometheus.Close)
	db := openK8sHTTPDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{
		WorkspaceID:        wsID.String(),
		ClusterID:          clusterID.String(),
		PrometheusURL:      prometheus.URL,
		PrometheusUser:     "prom-user",
		PrometheusPassword: "prom-secret",
	}).Error)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "prod"}}},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	rec := performJSON(engine, http.MethodPost, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/metrics", wsID, map[string]string{
		"template": "node_cpu",
		"range":    "5m",
		"node":     "node-1",
	})

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), "PLUGIN_PROMETHEUS_UPSTREAM")
	require.NotContains(t, rec.Body.String(), "prom-secret")
	require.NotContains(t, rec.Body.String(), "internal-prometheus")
	require.NotContains(t, rec.Body.String(), prometheus.URL)
}

func TestPostMetricsAcceptsOnlyPrometheusJSON(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	t.Cleanup(prometheus.Close)
	db := openK8sHTTPDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{
		WorkspaceID:   wsID.String(),
		ClusterID:     clusterID.String(),
		PrometheusURL: prometheus.URL,
	}).Error)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "prod"}}},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	rec := performJSON(engine, http.MethodPost, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/metrics", wsID, map[string]string{
		"template": "node_cpu",
		"range":    "5m",
		"node":     "node-1",
	})

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), "PLUGIN_PROMETHEUS_UPSTREAM")
}

func TestPostMetricsPassesPrometheusVectorJSON(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result":     []any{},
			},
		})
	}))
	t.Cleanup(prometheus.Close)
	db := openK8sHTTPDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{
		WorkspaceID:   wsID.String(),
		ClusterID:     clusterID.String(),
		PrometheusURL: prometheus.URL,
	}).Error)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "prod"}}},
		Members:  allowMembers{grants: map[kernel.WorkspaceID]map[kernel.MemberID]bool{wsID: {memberID: true}}},
	})

	rec := performJSON(engine, http.MethodPost, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/metrics", wsID, map[string]string{
		"template": "node_cpu",
		"range":    "5m",
		"node":     "node-1",
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"resultType":"vector"`)
}

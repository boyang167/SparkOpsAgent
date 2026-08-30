package http

import (
	"bytes"
	"context"
	"encoding/json"
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
		Writer: allowWriter{allow: true},
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

func TestPutClusterSettingsRejectsUngrantCluster(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openK8sHTTPDB(t)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: nil},
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
		ClusterID:          clusterID.String(),
		PrometheusPassword: "existing-prom",
		VelauxPassword:     "existing-velaux",
		LLMAPIKey:          "existing-key",
	}).Error)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "prod"}}},
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
	require.NoError(t, db.First(&stored, "cluster_id = ?", clusterID.String()).Error)
	require.Equal(t, "https://prometheus.example", stored.PrometheusURL)
	require.Equal(t, "prom-user", stored.PrometheusUser)
	require.Equal(t, "existing-prom", stored.PrometheusPassword)
	require.Equal(t, "", stored.VelauxURL)
	require.Equal(t, "velaux-user", stored.VelauxUser)
	require.Equal(t, "", stored.VelauxPassword)
	require.Equal(t, "https://llm.example/v1", stored.LLMBaseURL)
	require.Equal(t, "existing-key", stored.LLMAPIKey)
	require.Equal(t, "qwen-plus", stored.LLMModel)
}

func TestPutClusterSettingsValidatesURLFields(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openK8sHTTPDB(t)
	engine := testEngine(memberID, sparkplugin.PluginContext{
		DB:       db,
		Clusters: fakeClusterFactory{items: []sparkplugin.ClusterMeta{{ID: clusterID.String(), Name: "prod"}}},
		Writer:   allowWriter{allow: true},
	})

	for _, payload := range []map[string]string{
		{"prometheus_url": "ftp://prometheus.example"},
		{"velaux_url": "http://[::1"},
		{"llm_base_url": "//llm.example"},
	} {
		rec := performJSON(engine, http.MethodPut, "/api/v1/plugins/kubernetes/clusters/"+clusterID.String()+"/settings", wsID, payload)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "PLUGIN_CLUSTER_SETTINGS_INVALID_URL")
	}
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
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	engine.Use(httpx.RequireWorkspace())
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
	items []sparkplugin.ClusterMeta
}

func (f fakeClusterFactory) ForWorkspace(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	return nil, nil, nil, nil, nil
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

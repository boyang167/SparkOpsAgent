package chat

import (
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestQueryRequiresAdminBeforeClusterAndLLM(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openChatDB(t)
	factory := &recordingClusterFactory{}
	engine := chatEngine(memberID, NewHandler(sparkplugin.PluginContext{
		DB:       db,
		Clusters: factory,
		Writer:   allowWriter{allow: false},
	}))

	rec := performJSON(engine, http.MethodPost, "/clusters/"+clusterID.String()+"/chat/query", wsID, map[string]string{"prompt": "hello"})

	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	require.False(t, factory.called)
}

func TestQueryRejectsPromptOverrides(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openChatDB(t)
	factory := &recordingClusterFactory{}
	engine := chatEngine(memberID, NewHandler(sparkplugin.PluginContext{
		DB:       db,
		Clusters: factory,
		Writer:   allowWriter{allow: true},
	}))

	rec := performRaw(engine, "/clusters/"+clusterID.String()+"/chat/query", wsID, []byte(`{"prompt":"hello","model":"evil"}`))

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "PLUGIN_CHAT_REQUEST_INVALID")
	require.False(t, factory.called)
}

func TestQueryStreamsOpenAICompatibleSSEWithSanitizedPrompt(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	var upstreamPath, upstreamAuth string
	var upstreamBody []byte
	audit := &recordingAudit{}
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAuth = r.Header.Get("Authorization")
		upstreamBody, _ = ioReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(llm.Close)

	db := openChatDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{
		ClusterID:  clusterID.String(),
		LLMBaseURL: llm.URL + "/v1",
		LLMAPIKey:  "llm-secret",
		LLMModel:   "qwen-plus",
	}).Error)
	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "secret-ns"}},
	)
	engine := chatEngine(memberID, NewHandler(sparkplugin.PluginContext{
		DB:    db,
		Audit: audit,
		Clusters: &recordingClusterFactory{
			client: client,
			meta:   &sparkplugin.ClusterMeta{ID: clusterID.String(), Name: "prod", Server: "https://secret-server"},
			restConfig: &rest.Config{
				Host:        "https://secret-server",
				BearerToken: "cluster-token",
				TLSClientConfig: rest.TLSClientConfig{
					CAData:   []byte("ca-secret"),
					CertData: []byte("cert-secret"),
					KeyData:  []byte("key-secret"),
				},
			},
		},
		Writer: allowWriter{allow: true},
	}))

	rec := performJSON(engine, http.MethodPost, "/clusters/"+clusterID.String()+"/chat/query", wsID, map[string]string{"prompt": "summarize"})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "/v1/chat/completions", upstreamPath)
	require.Equal(t, "Bearer llm-secret", upstreamAuth)
	body := string(upstreamBody)
	require.Contains(t, body, `"model":"qwen-plus"`)
	require.Contains(t, body, "summarize")
	require.Contains(t, body, clusterID.String())
	require.Contains(t, body, "prod")
	require.Contains(t, body, "namespace_count=2")
	for _, secret := range []string{"secret-server", "cluster-token", "ca-secret", "cert-secret", "key-secret", "llm-secret", "secret-ns"} {
		require.NotContains(t, body, secret)
		require.NotContains(t, rec.Body.String(), secret)
	}
	require.Contains(t, rec.Body.String(), "event: chunk")
	require.Contains(t, rec.Body.String(), "event: done")
	require.Len(t, audit.events, 2)
	require.Equal(t, "plugin.kubernetes.chat.started", audit.events[0].Action)
	require.Equal(t, "plugin.kubernetes.chat.completed", audit.events[1].Action)
	auditBody, err := json.Marshal(audit.events)
	require.NoError(t, err)
	require.Contains(t, string(auditBody), `"model":"qwen-plus"`)
	require.Contains(t, string(auditBody), clusterID.String())
	for _, secret := range []string{"summarize", "llm-secret", llm.URL, "secret-server", "cluster-token", "ok"} {
		require.NotContains(t, string(auditBody), secret)
	}
}

func TestQueryRejectsNonEventStreamUpstreamBeforeLocalStream(t *testing.T) {
	for _, contentType := range []string{"application/json", "text/html; charset=utf-8"} {
		t.Run(contentType, func(t *testing.T) {
			wsID := kernel.NewWorkspaceID()
			memberID := kernel.NewMemberID()
			clusterID := kernel.NewClusterInfoID()
			audit := &recordingAudit{}
			llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", contentType)
				_, _ = w.Write([]byte(`{"secret":"response-secret"}`))
			}))
			t.Cleanup(llm.Close)
			db := openChatDB(t)
			require.NoError(t, db.Create(&k8sschema.ClusterSettings{
				ClusterID:  clusterID.String(),
				LLMBaseURL: llm.URL,
				LLMAPIKey:  "llm-secret",
				LLMModel:   "qwen-plus",
			}).Error)
			engine := chatEngine(memberID, NewHandler(sparkplugin.PluginContext{
				DB:       db,
				Audit:    audit,
				Clusters: &recordingClusterFactory{client: fake.NewSimpleClientset(), meta: &sparkplugin.ClusterMeta{ID: clusterID.String(), Name: "prod"}},
				Writer:   allowWriter{allow: true},
			}))

			rec := performJSON(engine, http.MethodPost, "/clusters/"+clusterID.String()+"/chat/query", wsID, map[string]string{"prompt": "prompt-secret"})

			require.Equal(t, http.StatusBadGateway, rec.Code, rec.Body.String())
			require.NotContains(t, rec.Header().Get("Content-Type"), "text/event-stream")
			require.NotContains(t, rec.Body.String(), "response-secret")
			require.Len(t, audit.events, 2)
			require.Equal(t, "plugin.kubernetes.chat.completed", audit.events[1].Action)
			auditBody, err := json.Marshal(audit.events)
			require.NoError(t, err)
			for _, secret := range []string{"prompt-secret", "llm-secret", llm.URL, "response-secret"} {
				require.NotContains(t, string(auditBody), secret)
			}
		})
	}
}

func TestQueryReturnsNotConfiguredWithoutCallingLLM(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	db := openChatDB(t)
	called := false
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	t.Cleanup(llm.Close)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{ClusterID: clusterID.String(), LLMBaseURL: llm.URL, LLMModel: "qwen"}).Error)
	engine := chatEngine(memberID, NewHandler(sparkplugin.PluginContext{
		DB:       db,
		Clusters: &recordingClusterFactory{client: fake.NewSimpleClientset(), meta: &sparkplugin.ClusterMeta{ID: clusterID.String(), Name: "prod"}},
		Writer:   allowWriter{allow: true},
	}))

	rec := performJSON(engine, http.MethodPost, "/clusters/"+clusterID.String()+"/chat/query", wsID, map[string]string{"prompt": "hello"})

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "PLUGIN_LLM_NOT_CONFIGURED")
	require.False(t, called)
}

func TestQueryPropagatesClientCancelToUpstream(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	started := make(chan struct{}, 1)
	seenCanceled := make(chan struct{}, 1)
	audit := &recordingAudit{}
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		started <- struct{}{}
		<-r.Context().Done()
		seenCanceled <- struct{}{}
		return nil, r.Context().Err()
	})}
	db := openChatDB(t)
	require.NoError(t, db.Create(&k8sschema.ClusterSettings{
		ClusterID:  clusterID.String(),
		LLMBaseURL: "https://llm.example/v1",
		LLMAPIKey:  "llm-secret",
		LLMModel:   "qwen-plus",
	}).Error)
	engine := chatEngine(memberID, NewHandlerWithClient(sparkplugin.PluginContext{
		DB:       db,
		Audit:    audit,
		Clusters: &recordingClusterFactory{client: fake.NewSimpleClientset(), meta: &sparkplugin.ClusterMeta{ID: clusterID.String(), Name: "prod"}},
		Writer:   allowWriter{allow: true},
	}, httpClient))
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/clusters/"+clusterID.String()+"/chat/query", strings.NewReader(`{"prompt":"hello"}`))
	req.Header.Set("X-Workspace-Id", wsID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.ServeHTTP(rec, req)
	}()
	<-started
	cancel()

	select {
	case <-seenCanceled:
	case <-time.After(time.Second):
		t.Fatal("upstream request did not observe client cancellation")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not return after client cancellation")
	}
	require.Len(t, audit.events, 2)
	require.Equal(t, "plugin.kubernetes.chat.completed", audit.events[1].Action)
}

func openChatDB(t *testing.T) *gorm.DB {
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

func chatEngine(memberID kernel.MemberID, handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		authn.SetMemberIDForTest(c, memberID)
		c.Next()
	})
	engine.Use(httpx.RequireWorkspace())
	engine.POST("/clusters/:clusterId/chat/query", handler.Query)
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

func performRaw(engine *gin.Engine, path string, wsID kernel.WorkspaceID, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("X-Workspace-Id", wsID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func ioReadAll(r io.Reader) ([]byte, error) {
	defer func() {
		if closer, ok := r.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	return buf.Bytes(), err
}

type recordingClusterFactory struct {
	called     bool
	client     kubernetes.Interface
	restConfig *rest.Config
	meta       *sparkplugin.ClusterMeta
}

func (f *recordingClusterFactory) ForWorkspace(context.Context, kernel.WorkspaceID, kernel.ClusterInfoID) (
	kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error,
) {
	f.called = true
	client := f.client
	if client == nil {
		client = fake.NewSimpleClientset()
	}
	meta := f.meta
	if meta == nil {
		meta = &sparkplugin.ClusterMeta{}
	}
	return client, nil, f.restConfig, meta, nil
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type recordingAudit struct {
	events []sparkplugin.AuditEvent
	err    error
}

func (a *recordingAudit) Record(_ context.Context, event sparkplugin.AuditEvent) error {
	a.events = append(a.events, event)
	return a.err
}

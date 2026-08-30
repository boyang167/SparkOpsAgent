package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	maxPromptBytes      = 16 << 10
	namespaceListLimit  = int64(500)
	namespaceTimeout    = 5 * time.Second
	llmTimeout          = 60 * time.Second
	maxUpstreamLineSize = 64 << 10
	maxUpstreamBytes    = 1 << 20
)

type Handler struct {
	ctx    sparkplugin.PluginContext
	client *http.Client
}

type queryRequest struct {
	Prompt string `json:"prompt"`
}

func NewHandler(ctx sparkplugin.PluginContext) *Handler {
	return NewHandlerWithClient(ctx, nil)
}

func NewHandlerWithClient(ctx sparkplugin.PluginContext, client *http.Client) *Handler {
	if client == nil {
		client = &http.Client{
			Timeout: llmTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &Handler{ctx: ctx, client: client}
}

func (h *Handler) Query(c *gin.Context) {
	if h == nil || h.ctx.DB == nil || h.ctx.Clusters == nil || h.ctx.Writer == nil || h.client == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return
	}
	wsID, memberID, ok := requireWrite(c.Request.Context(), c, h.ctx.Writer)
	if !ok {
		return
	}
	clusterID, err := kernel.ParseClusterInfoID(c.Param("clusterId"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_INVALID", "invalid cluster id", http.StatusBadRequest)
		return
	}
	var req queryRequest
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, maxPromptBytes+1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || len(req.Prompt) == 0 || len([]byte(req.Prompt)) > maxPromptBytes {
		httpx.Fail(c, "PLUGIN_CHAT_REQUEST_INVALID", "invalid chat request", http.StatusBadRequest)
		return
	}
	clientset, _, _, meta, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	if meta == nil {
		meta = &sparkplugin.ClusterMeta{ID: clusterID.String()}
	}
	namespaceCount, err := countNamespaces(c.Request.Context(), clientset)
	if err != nil {
		httpx.Fail(c, "PLUGIN_CHAT_CLUSTER_SUMMARY_FAILED", "cluster summary unavailable", http.StatusBadGateway)
		return
	}
	settings, err := h.settings(c.Request.Context(), clusterID)
	if err != nil || strings.TrimSpace(settings.LLMBaseURL) == "" || strings.TrimSpace(settings.LLMAPIKey) == "" || strings.TrimSpace(settings.LLMModel) == "" {
		httpx.Fail(c, "PLUGIN_LLM_NOT_CONFIGURED", "LLM is not configured", http.StatusBadRequest)
		return
	}
	upstreamReq, err := buildUpstreamRequest(c.Request.Context(), settings, clusterSummary(*meta, clusterID, namespaceCount), req.Prompt)
	if err != nil {
		httpx.Fail(c, "PLUGIN_LLM_NOT_CONFIGURED", "LLM is not configured", http.StatusBadRequest)
		return
	}
	h.recordChatAudit(c.Request.Context(), "plugin.kubernetes.chat.started", wsID, memberID, clusterID, settings.LLMModel)
	defer h.recordChatAudit(context.Background(), "plugin.kubernetes.chat.completed", wsID, memberID, clusterID, settings.LLMModel)
	resp, err := h.client.Do(upstreamReq)
	if err != nil {
		if errors.Is(c.Request.Context().Err(), context.Canceled) {
			return
		}
		httpx.Fail(c, "PLUGIN_CHAT_UPSTREAM_UNAVAILABLE", "chat upstream unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		httpx.Fail(c, "PLUGIN_CHAT_UPSTREAM_UNAVAILABLE", "chat upstream unavailable", http.StatusBadGateway)
		return
	}
	if !isEventStreamContentType(resp.Header.Get("Content-Type")) {
		httpx.Fail(c, "PLUGIN_CHAT_UPSTREAM_UNAVAILABLE", "chat upstream unavailable", http.StatusBadGateway)
		return
	}
	writer := prepareSSE(c)
	if err := streamOpenAISSE(c.Request.Context(), writer, resp.Body); err != nil && !errors.Is(c.Request.Context().Err(), context.Canceled) {
		_ = writer.Event("error", "chat stream failed")
	}
}

func (h *Handler) recordChatAudit(ctx context.Context, action string, wsID kernel.WorkspaceID, memberID kernel.MemberID, clusterID kernel.ClusterInfoID, model string) {
	if h == nil || h.ctx.Audit == nil {
		return
	}
	detail := map[string]any{
		"workspace_id": wsID.String(),
		"member_id":    memberID.String(),
		"cluster_id":   clusterID.String(),
		"model":        strings.TrimSpace(model),
	}
	err := h.ctx.Audit.Record(ctx, sparkplugin.AuditEvent{
		WorkspaceID: wsID.String(),
		MemberID:    memberID.String(),
		Action:      action,
		Detail:      detail,
	})
	if err != nil && h.ctx.Logger != nil {
		h.ctx.Logger.Warn("kubernetes chat audit failed")
	}
}

func (h *Handler) settings(ctx context.Context, clusterID kernel.ClusterInfoID) (k8sschema.ClusterSettings, error) {
	var settings k8sschema.ClusterSettings
	err := h.ctx.DB.WithContext(ctx).First(&settings, "cluster_id = ?", clusterID.String()).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return k8sschema.ClusterSettings{}, err
	}
	return settings, err
}

func countNamespaces(ctx context.Context, client kubernetes.Interface) (int, error) {
	if client == nil {
		return 0, errors.New("kubernetes client unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, namespaceTimeout)
	defer cancel()
	total := 0
	var cont string
	for {
		list, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: namespaceListLimit, Continue: cont})
		if err != nil {
			return 0, err
		}
		total += len(list.Items)
		cont = list.Continue
		if cont == "" {
			return total, nil
		}
	}
}

func clusterSummary(meta sparkplugin.ClusterMeta, clusterID kernel.ClusterInfoID, namespaceCount int) string {
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		name = "unknown"
	}
	id := strings.TrimSpace(meta.ID)
	if id == "" {
		id = clusterID.String()
	}
	return "cluster_id=" + id + "\ncluster_name=" + name + "\nnamespace_count=" + strconv.Itoa(namespaceCount)
}

func buildUpstreamRequest(ctx context.Context, settings k8sschema.ClusterSettings, summary, prompt string) (*http.Request, error) {
	endpoint, err := llmEndpoint(settings.LLMBaseURL)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"model":  strings.TrimSpace(settings.LLMModel),
		"stream": true,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a Kubernetes assistant. Use only this read-only cluster summary; never request or expose kubeconfig, certificates, tokens, or settings secrets.\n" + summary,
			},
			{"role": "user", "content": prompt},
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(settings.LLMAPIKey))
	return req, nil
}

func llmEndpoint(baseURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return "", errors.New("invalid llm url")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("invalid llm url")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/chat/completions"
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.User = nil
	return parsed.String(), nil
}

func streamOpenAISSE(ctx context.Context, writer *sseWriter, body io.Reader) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 4096), maxUpstreamLineSize)
	total := 0
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Text()
		total += len(line)
		if total > maxUpstreamBytes {
			return errors.New("chat stream too large")
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			return writer.Event("done", map[string]bool{"ok": true})
		}
		if err := writer.Event("chunk", payload); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return writer.Event("done", map[string]bool{"ok": true})
}

func isEventStreamContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "text/event-stream")
}

func requireWrite(ctx context.Context, c *gin.Context, writer sparkplugin.WriteAuthorizer) (kernel.WorkspaceID, kernel.MemberID, bool) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	allowed, err := writer.CanWrite(ctx, wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	if !allowed {
		httpx.Fail(c, "GOVERNANCE_API_ACCESS_DENIED", "governance admin permission required", http.StatusForbidden)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	return wsID, memberID, true
}

func handleClusterError(c *gin.Context, err error) {
	if de, ok := kernel.IsDomainError(err); ok {
		switch de.Code() {
		case "PLUGIN_CLUSTER_FORBIDDEN":
			httpx.Fail(c, de.Code(), "cluster is not granted to workspace", http.StatusForbidden)
			return
		case "PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE":
			httpx.Fail(c, de.Code(), "cluster credentials are incomplete", http.StatusServiceUnavailable)
			return
		}
	}
	if apierrors.IsUnauthorized(err) || apierrors.IsForbidden(err) {
		httpx.Fail(c, "PLUGIN_CLUSTER_FORBIDDEN", "cluster is not granted to workspace", http.StatusForbidden)
		return
	}
	httpx.Fail(c, "PLUGIN_CHAT_UPSTREAM_UNAVAILABLE", "chat upstream unavailable", http.StatusBadGateway)
}

type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
}

func prepareSSE(c *gin.Context) *sseWriter {
	header := c.Writer.Header()
	header.Set("Content-Type", "text/event-stream; charset=utf-8")
	header.Set("Cache-Control", "no-cache, no-transform")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher, _ := c.Writer.(http.Flusher)
	return &sseWriter{w: c.Writer, flusher: flusher}
}

func (w *sseWriter) Event(event string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := io.WriteString(w.w, "event: "+event+"\n"); err != nil {
		return err
	}
	if _, err := io.WriteString(w.w, "data: "+string(payload)+"\n\n"); err != nil {
		return err
	}
	if w.flusher != nil {
		w.flusher.Flush()
	}
	return nil
}

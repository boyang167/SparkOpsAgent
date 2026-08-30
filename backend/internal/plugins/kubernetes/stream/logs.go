package stream

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
)

const (
	maxLogTailLines    int64 = 10000
	maxLogSinceSeconds int64 = 604800
	logChunkBytes            = 32 << 10
)

type LogRequest struct {
	Namespace    string
	Pod          string
	Container    string
	TailLines    *int64
	SinceSeconds *int64
}

type LogStreamer interface {
	StreamLogs(ctx context.Context, client kubernetes.Interface, req LogRequest) (io.ReadCloser, error)
}

type KubernetesLogStreamer struct{}

func (KubernetesLogStreamer) StreamLogs(ctx context.Context, client kubernetes.Interface, req LogRequest) (io.ReadCloser, error) {
	if client == nil {
		return nil, errors.New("kubernetes client unavailable")
	}
	return client.CoreV1().Pods(req.Namespace).GetLogs(req.Pod, &corev1.PodLogOptions{
		Container:    req.Container,
		Follow:       true,
		TailLines:    req.TailLines,
		SinceSeconds: req.SinceSeconds,
	}).Stream(ctx)
}

type Handler struct {
	ctx  sparkplugin.PluginContext
	logs LogStreamer
	exec ExecStreamer
}

func NewHandler(ctx sparkplugin.PluginContext) *Handler {
	return NewHandlerWithStreamers(ctx, KubernetesLogStreamer{}, KubernetesExecStreamer{})
}

func NewHandlerWithStreamers(ctx sparkplugin.PluginContext, logs LogStreamer, exec ExecStreamer) *Handler {
	if logs == nil {
		logs = KubernetesLogStreamer{}
	}
	if exec == nil {
		exec = KubernetesExecStreamer{}
	}
	return &Handler{ctx: ctx, logs: logs, exec: exec}
}

func (h *Handler) Logs(c *gin.Context) {
	if h == nil || h.ctx.Clusters == nil || h.logs == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return
	}
	target, ok := streamTargetFromRequest(c)
	if !ok {
		return
	}
	logReq, ok := logRequestFromQuery(c, target)
	if !ok {
		return
	}
	client, _, _, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), target.WorkspaceID, target.ClusterID)
	if err != nil {
		handleStreamClusterError(c, err)
		return
	}
	body, err := h.logs.StreamLogs(c.Request.Context(), client, logReq)
	if err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_STREAM_UNAVAILABLE", "pod log stream unavailable", http.StatusBadGateway)
		return
	}
	defer func() { _ = body.Close() }()

	writer := prepareSSE(c)
	if err := writer.Event("open", map[string]bool{"ok": true}); err != nil {
		return
	}
	buf := make([]byte, logChunkBytes)
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			if err := writer.Event("log", string(buf[:n])); err != nil {
				return
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return
			}
			if c.Request.Context().Err() != nil {
				return
			}
			_ = writer.Event("error", "pod log stream failed")
			return
		}
	}
}

type streamTarget struct {
	WorkspaceID kernel.WorkspaceID
	MemberID    kernel.MemberID
	ClusterID   kernel.ClusterInfoID
	Namespace   string
	Pod         string
	Container   string
}

func streamTargetFromRequest(c *gin.Context) (streamTarget, bool) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return streamTarget{}, false
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return streamTarget{}, false
	}
	clusterID, err := kernel.ParseClusterInfoID(c.Param("clusterId"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_INVALID", "invalid cluster id", http.StatusBadRequest)
		return streamTarget{}, false
	}
	namespace := c.Param("namespace")
	pod := c.Param("pod")
	container := c.Query("container")
	if !validDNSLabel(namespace) || !validDNSSubdomain(pod) || (container != "" && !validDNSLabel(container)) {
		httpx.Fail(c, "PLUGIN_KUBERNETES_STREAM_TARGET_INVALID", "invalid pod stream target", http.StatusBadRequest)
		return streamTarget{}, false
	}
	return streamTarget{
		WorkspaceID: wsID,
		MemberID:    memberID,
		ClusterID:   clusterID,
		Namespace:   namespace,
		Pod:         pod,
		Container:   container,
	}, true
}

func logRequestFromQuery(c *gin.Context, target streamTarget) (LogRequest, bool) {
	tailLines, ok := optionalBoundedInt64(c, "tailLines", maxLogTailLines)
	if !ok {
		return LogRequest{}, false
	}
	sinceSeconds, ok := optionalBoundedInt64(c, "sinceSeconds", maxLogSinceSeconds)
	if !ok {
		return LogRequest{}, false
	}
	return LogRequest{
		Namespace:    target.Namespace,
		Pod:          target.Pod,
		Container:    target.Container,
		TailLines:    tailLines,
		SinceSeconds: sinceSeconds,
	}, true
}

func optionalBoundedInt64(c *gin.Context, key string, max int64) (*int64, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		httpx.Fail(c, "PLUGIN_KUBERNETES_STREAM_QUERY_INVALID", "invalid stream query", http.StatusBadRequest)
		return nil, false
	}
	if value > max {
		value = max
	}
	return &value, true
}

type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
}

func newSSEWriter(w http.ResponseWriter) *sseWriter {
	flusher, _ := w.(http.Flusher)
	return &sseWriter{w: w, flusher: flusher}
}

func prepareSSE(c *gin.Context) *sseWriter {
	header := c.Writer.Header()
	header.Set("Content-Type", "text/event-stream; charset=utf-8")
	header.Set("Cache-Control", "no-cache, no-transform")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	return newSSEWriter(c.Writer)
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

func validDNSLabel(value string) bool {
	return len(validation.IsDNS1123Label(value)) == 0
}

func validDNSSubdomain(value string) bool {
	return len(validation.IsDNS1123Subdomain(value)) == 0
}

func handleStreamClusterError(c *gin.Context, err error) {
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
	httpx.Fail(c, "PLUGIN_KUBERNETES_STREAM_UPSTREAM_UNAVAILABLE", "kubernetes stream upstream unavailable", http.StatusBadGateway)
}

package diagnosis

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	k8sresource "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/resource"
	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	DefaultSessionTTL = 30 * time.Minute
	cleanupTimeout    = 10 * time.Second
)

var (
	ErrArchiveInvalid = errors.New("arthas archive invalid")
	ErrExecFailed     = errors.New("arthas exec failed")
)

type ExecRequest struct {
	Namespace string
	Pod       string
	Container string
	Command   []string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
}

type Executor interface {
	Exec(ctx context.Context, client kubernetes.Interface, restCfg *rest.Config, req ExecRequest) error
}

type KubernetesExecutor struct{}

func (KubernetesExecutor) Exec(ctx context.Context, client kubernetes.Interface, restCfg *rest.Config, req ExecRequest) error {
	if client == nil || restCfg == nil {
		return ErrExecFailed
	}
	restReq := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(req.Pod).
		Namespace(req.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: req.Container,
			Command:   append([]string(nil), req.Command...),
			Stdin:     req.Stdin != nil,
			Stdout:    req.Stdout != nil,
			Stderr:    req.Stderr != nil,
			TTY:       false,
		}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(restCfg, http.MethodPost, restReq.URL())
	if err != nil {
		return err
	}
	return executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  req.Stdin,
		Stdout: req.Stdout,
		Stderr: req.Stderr,
		Tty:    false,
	})
}

type Options struct {
	Executor     Executor
	Archive      []byte
	Now          func() time.Time
	RunAsync     func(func())
	PortForward  PortForwardStarter
}

type Handler struct {
	ctx         sparkplugin.PluginContext
	executor    Executor
	archive     []byte
	now         func() time.Time
	runAsync    func(func())
	portForward PortForwardStarter
}

type startRequest struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
	PID       int    `json:"pid"`
}

func NewHandler(ctx sparkplugin.PluginContext) *Handler {
	return NewHandlerWithOptions(ctx, Options{})
}

func NewHandlerWithOptions(ctx sparkplugin.PluginContext, opts Options) *Handler {
	executor := opts.Executor
	if executor == nil {
		executor = KubernetesExecutor{}
	}
	archive := opts.Archive
	if archive == nil {
		archive = k8sresource.ArthasArchive
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	runAsync := opts.RunAsync
	if runAsync == nil {
		runAsync = func(fn func()) { go fn() }
	}
	return &Handler{ctx: ctx, executor: executor, archive: archive, now: now, runAsync: runAsync, portForward: opts.PortForward}
}

func (h *Handler) Start(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	wsID, _, ok := requireWrite(c.Request.Context(), c, h.ctx.Writer)
	if !ok {
		return
	}
	clusterID, ok := clusterIDFromParam(c)
	if !ok {
		return
	}
	var req startRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || !validTarget(req.Namespace, req.Pod, req.Container) || req.PID <= 0 {
		httpx.Fail(c, "PLUGIN_ARTHAS_REQUEST_INVALID", "invalid arthas diagnosis request", http.StatusBadRequest)
		return
	}
	client, _, restCfg, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	h.cleanupExpired(c.Request.Context(), wsID, clusterID, client, restCfg)
	id := uuid.NewString()
	sessionDir := path.Join("/tmp", "sparkops-arthas-"+id)
	jarPath, err := arthasBootJarPath(h.archive, sessionDir)
	if err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_ARCHIVE_INVALID", "arthas archive invalid", http.StatusInternalServerError)
		return
	}
	now := h.now().UTC()
	row := k8sschema.Diagnosis{
		ID:          id,
		WorkspaceID: wsID.String(),
		ClusterID:   clusterID.String(),
		Namespace:   req.Namespace,
		Pod:         req.Pod,
		Container:   req.Container,
		PID:         req.PID,
		SessionDir:  sessionDir,
		JarPath:     jarPath,
		Status:      "starting",
		CreatedAt:   now,
		ExpiresAt:   now.Add(DefaultSessionTTL),
	}
	if err := h.ctx.DB.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_START_FAILED", "arthas diagnosis session start failed", http.StatusBadGateway)
		return
	}
	if err := h.exec(c.Request.Context(), client, restCfg, row, []string{"mkdir", "-p", sessionDir}, nil, nil, nil); err != nil {
		h.markFailedAndCleanup(row, client, restCfg)
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_START_FAILED", "arthas diagnosis session start failed", http.StatusBadGateway)
		return
	}
	if err := h.exec(c.Request.Context(), client, restCfg, row, []string{"tar", "-xmf", "-", "-C", sessionDir}, bytes.NewReader(h.archive), nil, nil); err != nil {
		h.markFailedAndCleanup(row, client, restCfg)
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_START_FAILED", "arthas diagnosis session start failed", http.StatusBadGateway)
		return
	}
	row.Status = "ready"
	if err := h.ctx.DB.WithContext(c.Request.Context()).Save(&row).Error; err != nil {
		h.markFailedAndCleanup(row, client, restCfg)
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_START_FAILED", "arthas diagnosis session start failed", http.StatusBadGateway)
		return
	}
	httpx.OK(c, gin.H{"id": row.ID, "status": row.Status, "expires_at": row.ExpiresAt})
}

func (h *Handler) Stream(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	wsID, _, ok := requireWrite(c.Request.Context(), c, h.ctx.Writer)
	if !ok {
		return
	}
	row, ok := h.loadSession(c, wsID)
	if !ok {
		return
	}
	clusterID, err := kernel.ParseClusterInfoID(row.ClusterID)
	if err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_NOT_FOUND", "arthas diagnosis session not found", http.StatusNotFound)
		return
	}
	client, _, restCfg, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	if h.expired(row) {
		h.cleanupSession(row, client, restCfg)
		_ = h.ctx.DB.WithContext(c.Request.Context()).Delete(&row).Error
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_NOT_FOUND", "arthas diagnosis session not found", http.StatusNotFound)
		return
	}

	writer := prepareSSE(c)
	_ = writer.Event("open", map[string]bool{"ok": true})
	_ = h.ctx.DB.WithContext(c.Request.Context()).Model(&row).Update("status", "running").Error
	err = h.exec(c.Request.Context(), client, restCfg, row, []string{"java", "-jar", row.JarPath, strconv.Itoa(row.PID)}, nil, outputWriter{writer: writer, event: "output"}, outputWriter{writer: writer, event: "error"})
	status := "completed"
	if err != nil {
		if errors.Is(c.Request.Context().Err(), context.Canceled) {
			status = "canceled"
		} else {
			status = "failed"
			_ = writer.Event("error", "arthas diagnosis stream failed")
		}
	}
	_ = h.ctx.DB.WithContext(context.Background()).Model(&row).Update("status", status).Error
	h.backgroundCleanup(row, client, restCfg)
	if status == "completed" {
		_ = writer.Event("exit", map[string]int{"code": 0})
	}
}

func (h *Handler) Delete(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	wsID, _, ok := requireWrite(c.Request.Context(), c, h.ctx.Writer)
	if !ok {
		return
	}
	row, ok := h.loadSession(c, wsID)
	if !ok {
		return
	}
	clusterID, err := kernel.ParseClusterInfoID(row.ClusterID)
	if err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_NOT_FOUND", "arthas diagnosis session not found", http.StatusNotFound)
		return
	}
	client, _, restCfg, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	h.cleanupSession(row, client, restCfg)
	if err := h.ctx.DB.WithContext(c.Request.Context()).Delete(&row).Error; err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_CLEANUP_FAILED", "arthas diagnosis cleanup failed", http.StatusBadGateway)
		return
	}
	httpx.OK(c, gin.H{"ok": true})
}

func (h *Handler) ready(c *gin.Context) bool {
	if h == nil || h.ctx.DB == nil || h.ctx.Clusters == nil || h.ctx.Writer == nil || h.executor == nil || len(h.archive) == 0 {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return false
	}
	return true
}

func (h *Handler) exec(ctx context.Context, client kubernetes.Interface, restCfg *rest.Config, row k8sschema.Diagnosis, command []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return h.executor.Exec(ctx, client, restCfg, ExecRequest{
		Namespace: row.Namespace,
		Pod:       row.Pod,
		Container: row.Container,
		Command:   command,
		Stdin:     stdin,
		Stdout:    stdout,
		Stderr:    stderr,
	})
}

func (h *Handler) markFailedAndCleanup(row k8sschema.Diagnosis, client kubernetes.Interface, restCfg *rest.Config) {
	_ = h.ctx.DB.WithContext(context.Background()).Model(&row).Update("status", "failed").Error
	h.backgroundCleanup(row, client, restCfg)
}

func (h *Handler) backgroundCleanup(row k8sschema.Diagnosis, client kubernetes.Interface, restCfg *rest.Config) {
	h.runAsync(func() {
		ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cancel()
		_ = h.cleanupSessionWithContext(ctx, row, client, restCfg)
	})
}

func (h *Handler) cleanupSession(row k8sschema.Diagnosis, client kubernetes.Interface, restCfg *rest.Config) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	_ = h.cleanupSessionWithContext(ctx, row, client, restCfg)
}

func (h *Handler) cleanupSessionWithContext(ctx context.Context, row k8sschema.Diagnosis, client kubernetes.Interface, restCfg *rest.Config) error {
	if strings.TrimSpace(row.SessionDir) == "" {
		return nil
	}
	return h.exec(ctx, client, restCfg, row, []string{"rm", "-rf", "--", row.SessionDir}, nil, nil, nil)
}

func (h *Handler) cleanupExpired(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID, client kubernetes.Interface, restCfg *rest.Config) {
	var rows []k8sschema.Diagnosis
	err := h.ctx.DB.WithContext(ctx).
		Where("workspace_id = ? AND cluster_id = ? AND expires_at < ?", wsID.String(), clusterID.String(), h.now().UTC()).
		Find(&rows).Error
	if err != nil {
		return
	}
	for _, row := range rows {
		h.cleanupSession(row, client, restCfg)
		_ = h.ctx.DB.WithContext(context.Background()).Delete(&row).Error
	}
}

func (h *Handler) loadSession(c *gin.Context, wsID kernel.WorkspaceID) (k8sschema.Diagnosis, bool) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_NOT_FOUND", "arthas diagnosis session not found", http.StatusNotFound)
		return k8sschema.Diagnosis{}, false
	}
	var row k8sschema.Diagnosis
	err := h.ctx.DB.WithContext(c.Request.Context()).First(&row, "id = ? AND workspace_id = ?", id, wsID.String()).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_NOT_FOUND", "arthas diagnosis session not found", http.StatusNotFound)
		return k8sschema.Diagnosis{}, false
	}
	if err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_LOAD_FAILED", "arthas diagnosis session load failed", http.StatusBadGateway)
		return k8sschema.Diagnosis{}, false
	}
	return row, true
}

func (h *Handler) expired(row k8sschema.Diagnosis) bool {
	return !row.ExpiresAt.IsZero() && row.ExpiresAt.Before(h.now().UTC())
}

func arthasBootJarPath(archive []byte, sessionDir string) (string, error) {
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", ErrArchiveInvalid
		}
		name := path.Clean(strings.TrimPrefix(strings.ReplaceAll(header.Name, "\\", "/"), "/"))
		if name == "." || strings.HasPrefix(name, "../") || strings.Contains(name, "/../") {
			return "", ErrArchiveInvalid
		}
		if path.Base(name) == "arthas-boot.jar" {
			return path.Join(sessionDir, name), nil
		}
	}
	return "", ErrArchiveInvalid
}

func validTarget(namespace, pod, container string) bool {
	return validDNSLabel(namespace) && validDNSSubdomain(pod) && validDNSLabel(container)
}

func validDNSLabel(value string) bool {
	return len(validation.IsDNS1123Label(value)) == 0
}

func validDNSSubdomain(value string) bool {
	return len(validation.IsDNS1123Subdomain(value)) == 0
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

func clusterIDFromParam(c *gin.Context) (kernel.ClusterInfoID, bool) {
	clusterID, err := kernel.ParseClusterInfoID(c.Param("clusterId"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_INVALID", "invalid cluster id", http.StatusBadRequest)
		return kernel.ClusterInfoID{}, false
	}
	return clusterID, true
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
	httpx.Fail(c, "PLUGIN_KUBERNETES_UPSTREAM_UNAVAILABLE", "kubernetes upstream unavailable", http.StatusBadGateway)
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

type outputWriter struct {
	writer *sseWriter
	event  string
}

func (w outputWriter) Write(p []byte) (int, error) {
	if err := w.writer.Event(w.event, string(bytes.ToValidUTF8(p, []byte("\uFFFD")))); err != nil {
		return 0, err
	}
	return len(p), nil
}

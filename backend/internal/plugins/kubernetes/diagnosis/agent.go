package diagnosis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	k8sschema "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/schema"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	arthasInstallRoot    = "/tmp"
	arthasBootPath       = "/tmp/arthas-bin/arthas-boot.jar"
	arthasClientPath     = "/tmp/arthas-bin/arthas-client.jar"
	maxExecOutput        = 1 << 20
)

type targetRequest struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
	ProcessID string `json:"process_id"`
}

type proxyRequest struct {
	targetRequest
	Body json.RawMessage `json:"body"`
}

type processInfo struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (h *Handler) Install(c *gin.Context) {
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
	req, ok := decodeTarget(c)
	if !ok {
		return
	}
	pid, ok := parseProcessID(req.ProcessID)
	if !ok {
		httpx.Fail(c, "PLUGIN_ARTHAS_REQUEST_INVALID", "invalid process id", http.StatusBadRequest)
		return
	}
	client, _, restCfg, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	row, err := h.findInstalled(c.Request.Context(), wsID, clusterID, req.Namespace, req.Pod)
	if err == nil && row.ID != "" {
		httpx.OK(c, gin.H{"diagnosis": publicDiagnosis(row)})
		return
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_LOAD_FAILED", "arthas diagnosis session load failed", http.StatusBadGateway)
		return
	}
	target := execTarget{WorkspaceID: wsID, ClusterID: clusterID, Namespace: req.Namespace, Pod: req.Pod, Container: req.Container}
	if _, ok := h.execCommand(c.Request.Context(), client, restCfg, target, []string{"java", "-version"}, nil); !ok {
		httpx.Fail(c, "PLUGIN_ARTHAS_JAVA_UNAVAILABLE", "container does not have a Java environment", http.StatusBadRequest)
		return
	}
	if _, ok := h.execCommand(c.Request.Context(), client, restCfg, target, []string{"mkdir", "-p", arthasInstallRoot}, nil); !ok {
		httpx.Fail(c, "PLUGIN_ARTHAS_INSTALL_FAILED", "arthas install failed", http.StatusBadGateway)
		return
	}
	if _, ok := h.execCommand(c.Request.Context(), client, restCfg, target, []string{"tar", "-xmf", "-", "-C", arthasInstallRoot}, bytes.NewReader(h.archive)); !ok {
		httpx.Fail(c, "PLUGIN_ARTHAS_INSTALL_FAILED", "arthas install failed", http.StatusBadGateway)
		return
	}
	startCmd := fmt.Sprintf("nohup java -jar %s %d --target-ip 0.0.0.0 > /dev/null 2>&1 &", arthasBootPath, pid)
	if _, ok := h.execCommand(c.Request.Context(), client, restCfg, target, []string{"/bin/sh", "-c", startCmd}, nil); !ok {
		httpx.Fail(c, "PLUGIN_ARTHAS_INSTALL_FAILED", "arthas install failed", http.StatusBadGateway)
		return
	}
	localPort, stop, err := h.waitAndForward(c.Request.Context(), client, restCfg, target)
	if err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_INSTALL_FAILED", "arthas port forward failed", http.StatusBadGateway)
		return
	}
	now := h.now().UTC()
	id := uuid.NewString()
	row = k8sschema.Diagnosis{
		ID:          id,
		WorkspaceID: wsID.String(),
		ClusterID:   clusterID.String(),
		Namespace:   req.Namespace,
		Pod:         req.Pod,
		Container:   req.Container,
		PID:         pid,
		Port:        arthasHTTPPort,
		LocalPort:   localPort,
		Status:      "installed",
		CreatedAt:   now,
		ExpiresAt:   now.Add(DefaultSessionTTL),
	}
	if err := h.ctx.DB.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
		if stop != nil {
			stop()
		}
		httpx.Fail(c, "PLUGIN_ARTHAS_INSTALL_FAILED", "arthas install failed", http.StatusBadGateway)
		return
	}
	registerPortForward(row.ID, localPort, stop)
	httpx.OK(c, gin.H{"diagnosis": publicDiagnosis(row)})
}

func (h *Handler) Uninstall(c *gin.Context) {
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
	req, ok := decodeTarget(c)
	if !ok {
		return
	}
	row, err := h.findInstalled(c.Request.Context(), wsID, clusterID, req.Namespace, req.Pod)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httpx.OK(c, gin.H{"ok": true})
		return
	}
	if err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_LOAD_FAILED", "arthas diagnosis session load failed", http.StatusBadGateway)
		return
	}
	client, _, restCfg, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	stopPortForward(row.ID)
	target := execTarget{WorkspaceID: wsID, ClusterID: clusterID, Namespace: req.Namespace, Pod: req.Pod, Container: req.Container}
	stopCmd := fmt.Sprintf("java -jar %s 127.0.0.1 3658 -c \"stop\" || true", arthasClientPath)
	_, _ = h.execCommand(c.Request.Context(), client, restCfg, target, []string{"/bin/sh", "-c", stopCmd}, nil)
	for _, path := range []string{"/tmp/logs", "/tmp/arthas-bin", "/arthas-output", "/tmp/arthas-bin.tar"} {
		_, _ = h.execCommand(c.Request.Context(), client, restCfg, target, []string{"rm", "-rf", "--", path}, nil)
	}
	if err := h.ctx.DB.WithContext(c.Request.Context()).Delete(&row).Error; err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_CLEANUP_FAILED", "arthas diagnosis cleanup failed", http.StatusBadGateway)
		return
	}
	httpx.OK(c, gin.H{"ok": true})
}

func (h *Handler) GetByPod(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	clusterID, ok := clusterIDFromParam(c)
	if !ok {
		return
	}
	req, ok := decodeTarget(c)
	if !ok {
		return
	}
	row, err := h.findInstalled(c.Request.Context(), wsID, clusterID, req.Namespace, req.Pod)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httpx.OK(c, gin.H{"diagnosis": gin.H{}})
		return
	}
	if err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_LOAD_FAILED", "arthas diagnosis session load failed", http.StatusBadGateway)
		return
	}
	httpx.OK(c, gin.H{"diagnosis": publicDiagnosis(row)})
}

func (h *Handler) Jps(c *gin.Context) {
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
	req, ok := decodeTarget(c)
	if !ok {
		return
	}
	client, _, restCfg, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), wsID, clusterID)
	if err != nil {
		handleClusterError(c, err)
		return
	}
	target := execTarget{WorkspaceID: wsID, ClusterID: clusterID, Namespace: req.Namespace, Pod: req.Pod, Container: req.Container}
	if _, ok := h.execCommand(c.Request.Context(), client, restCfg, target, []string{"java", "-version"}, nil); !ok {
		httpx.Fail(c, "PLUGIN_ARTHAS_JAVA_UNAVAILABLE", "container does not have a Java environment", http.StatusBadRequest)
		return
	}
	stdout, ok := h.execCommand(c.Request.Context(), client, restCfg, target, []string{"/bin/sh", "-c", "jps -l"}, nil)
	if !ok {
		httpx.Fail(c, "PLUGIN_ARTHAS_JPS_FAILED", "jps execution failed", http.StatusBadGateway)
		return
	}
	httpx.OK(c, gin.H{"diagnosis": parseJps(stdout.String())})
}

func (h *Handler) Proxy(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	clusterID, ok := clusterIDFromParam(c)
	if !ok {
		return
	}
	var req proxyRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || !validTarget(req.Namespace, req.Pod, req.Container) {
		httpx.Fail(c, "PLUGIN_ARTHAS_REQUEST_INVALID", "invalid arthas diagnosis request", http.StatusBadRequest)
		return
	}
	if len(req.Body) == 0 {
		httpx.Fail(c, "PLUGIN_ARTHAS_REQUEST_INVALID", "invalid arthas diagnosis request", http.StatusBadRequest)
		return
	}
	row, err := h.findInstalled(c.Request.Context(), wsID, clusterID, req.Namespace, req.Pod)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httpx.Fail(c, "PLUGIN_ARTHAS_NOT_INSTALLED", "arthas is not installed on target pod", http.StatusBadRequest)
		return
	}
	if err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_SESSION_LOAD_FAILED", "arthas diagnosis session load failed", http.StatusBadGateway)
		return
	}
	localPort := row.LocalPort
	if localPort <= 0 {
		if port, ok := localPortFor(row.ID); ok {
			localPort = port
		}
	}
	if localPort <= 0 {
		httpx.Fail(c, "PLUGIN_ARTHAS_NOT_INSTALLED", "arthas is not installed on target pod", http.StatusBadRequest)
		return
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/api", localPort)
	upstream, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, url, bytes.NewReader(req.Body))
	if err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_PROXY_FAILED", "arthas proxy request failed", http.StatusBadGateway)
		return
	}
	upstream.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(upstream)
	if err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_PROXY_FAILED", "arthas proxy request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxExecOutput))
	if err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_PROXY_FAILED", "arthas proxy request failed", http.StatusBadGateway)
		return
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		httpx.Fail(c, "PLUGIN_ARTHAS_PROXY_FAILED", "arthas proxy request failed", http.StatusBadGateway)
		return
	}
	httpx.OK(c, gin.H{"arthas": payload})
}

type execTarget struct {
	WorkspaceID kernel.WorkspaceID
	ClusterID   kernel.ClusterInfoID
	Namespace   string
	Pod         string
	Container   string
}

func (h *Handler) execCommand(ctx context.Context, client kubernetes.Interface, restCfg *rest.Config, target execTarget, command []string, stdin io.Reader) (*limitedBuffer, bool) {
	stdout := &limitedBuffer{limit: maxExecOutput}
	stderr := &limitedBuffer{limit: maxExecOutput}
	err := h.exec(ctx, client, restCfg, k8sschema.Diagnosis{
		Namespace: target.Namespace,
		Pod:       target.Pod,
		Container: target.Container,
	}, command, stdin, stdout, stderr)
	if err != nil {
		return nil, false
	}
	return stdout, true
}

func (h *Handler) waitAndForward(ctx context.Context, client kubernetes.Interface, restCfg *rest.Config, target execTarget) (localPort int, stop func(), err error) {
	starter := h.portForward
	if starter == nil {
		starter = KubernetesPortForwardStarter{}
	}
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		stdout, ok := h.execCommand(ctx, client, restCfg, target, []string{"/bin/sh", "-c", "cat /proc/net/tcp* 2>/dev/null | grep '"+arthasPortHexPattern+"' || true"}, nil)
		if ok && strings.Contains(stdout.String(), arthasPortHexPattern) {
			return starter.Start(ctx, client, restCfg, target.Namespace, target.Pod, arthasHTTPPort)
		}
		select {
		case <-ctx.Done():
			return 0, nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
	return 0, nil, errors.New("arthas port not ready")
}

func (h *Handler) findInstalled(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID, namespace, pod string) (k8sschema.Diagnosis, error) {
	var row k8sschema.Diagnosis
	err := h.ctx.DB.WithContext(ctx).
		Where("workspace_id = ? AND cluster_id = ? AND namespace = ? AND pod = ? AND port = ? AND status = ?",
			wsID.String(), clusterID.String(), namespace, pod, arthasHTTPPort, "installed").
		Order("created_at DESC").
		First(&row).Error
	return row, err
}

func decodeTarget(c *gin.Context) (targetRequest, bool) {
	var req targetRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || !validTarget(req.Namespace, req.Pod, req.Container) {
		httpx.Fail(c, "PLUGIN_ARTHAS_REQUEST_INVALID", "invalid arthas diagnosis request", http.StatusBadRequest)
		return targetRequest{}, false
	}
	return req, true
}

func parseProcessID(raw string) (int, bool) {
	pid, err := strconv.Atoi(strings.TrimSpace(raw))
	return pid, err == nil && pid > 0
}

func parseJps(output string) []processInfo {
	items := make([]processInfo, 0)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		items = append(items, processInfo{Name: strings.Join(parts[1:], " "), Value: parts[0]})
	}
	return items
}

func publicDiagnosis(row k8sschema.Diagnosis) gin.H {
	return gin.H{
		"id":          row.ID,
		"cluster_id":  row.ClusterID,
		"namespace":   row.Namespace,
		"pod_name":    row.Pod,
		"container":   row.Container,
		"local_port":  row.LocalPort,
		"remote_port": row.Port,
		"status":      row.Status,
	}
}

type limitedBuffer struct {
	limit int
	buf   bytes.Buffer
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 || b.buf.Len() >= b.limit {
		return len(p), nil
	}
	remain := b.limit - b.buf.Len()
	if len(p) > remain {
		p = p[:remain]
	}
	return b.buf.Write(p)
}

func (b *limitedBuffer) String() string {
	return b.buf.String()
}

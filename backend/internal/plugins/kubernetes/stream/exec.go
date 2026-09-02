package stream

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/records"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	clientgoexec "k8s.io/client-go/util/exec"
)

const (
	maxExecArgs       = 32
	maxExecArgBytes   = 4096
	maxExecTotalBytes = 16 << 10
	nodeTTYNamespace  = "kube-system"
	nodeTTYPodPrefix  = "sparkops-nodetty-"
	nodeTTYContainer  = "node-shell"
)

var nodeTTYExecPrefix = []string{"nsenter", "-t", "1", "-m", "-u", "-i", "-n", "--"}

type ExecRequest struct {
	Namespace string
	Pod       string
	Container string
	Command   []string
	Stdout    io.Writer
	Stderr    io.Writer
}

type ExecStreamer interface {
	StreamExec(ctx context.Context, client kubernetes.Interface, restCfg *rest.Config, req ExecRequest) error
}

type KubernetesExecStreamer struct{}

func (KubernetesExecStreamer) StreamExec(ctx context.Context, client kubernetes.Interface, restCfg *rest.Config, req ExecRequest) error {
	if client == nil || restCfg == nil {
		return errors.New("kubernetes exec unavailable")
	}
	restReq := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(req.Pod).
		Namespace(req.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: req.Container,
			Command:   append([]string(nil), req.Command...),
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(restCfg, http.MethodPost, restReq.URL())
	if err != nil {
		return err
	}
	return executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: req.Stdout,
		Stderr: req.Stderr,
		Tty:    false,
	})
}

func (h *Handler) Exec(c *gin.Context) {
	if h == nil || h.ctx.Clusters == nil || h.exec == nil || h.ctx.Writer == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return
	}
	target, ok := streamTargetFromRequest(c)
	if !ok {
		return
	}
	if _, _, ok := requireWrite(c.Request.Context(), c, h.ctx.Writer); !ok {
		return
	}
	command, ok := commandFromQuery(c)
	if !ok {
		return
	}
	command, ok = nodeTTYCommand(c, target, command)
	if !ok {
		return
	}
	client, _, restCfg, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), target.WorkspaceID, target.ClusterID)
	if err != nil {
		handleStreamClusterError(c, err)
		return
	}
	if restCfg == nil || strings.TrimSpace(restCfg.Host) == "" {
		httpx.Fail(c, "PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE", "cluster credentials are incomplete", http.StatusServiceUnavailable)
		return
	}

	writer := prepareSSE(c)
	if err := writer.Event("open", map[string]bool{"ok": true}); err != nil {
		return
	}
	recorder := h.startRecorder(c.Request.Context(), target, writer)
	defer h.closeRecorder(recorder)
	recordExecAudit(c.Request.Context(), h.ctx.Audit, target, "kubernetes.exec.started", "")
	var status string
	defer func() {
		recordExecAudit(context.Background(), h.ctx.Audit, target, "kubernetes.exec.ended", status)
	}()

	req := ExecRequest{
		Namespace: target.Namespace,
		Pod:       target.Pod,
		Container: target.Container,
		Command:   command,
		Stdout:    execOutputWriter(writer, recorder, "output", "stdout"),
		Stderr:    execOutputWriter(writer, recorder, "error", "stderr"),
	}
	err = h.exec.StreamExec(c.Request.Context(), client, restCfg, req)
	if err == nil {
		status = "completed"
		recordExit(recorder, 0)
		_ = writer.Event("exit", map[string]int{"code": 0})
		return
	}
	if exitErr, ok := err.(clientgoexec.CodeExitError); ok {
		status = "failed"
		code := exitErr.ExitStatus()
		recordExit(recorder, code)
		_ = writer.Event("exit", map[string]int{"code": code})
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		status = "canceled"
		return
	}
	status = "failed"
	_ = writer.Event("error", "pod exec stream failed")
	recordExit(recorder, 1)
	_ = writer.Event("exit", map[string]int{"code": 1})
}

func nodeTTYCommand(c *gin.Context, target streamTarget, command []string) ([]string, bool) {
	raw, present := c.GetQuery("node_tty")
	if !present {
		return command, true
	}
	if raw != "true" ||
		target.Namespace != nodeTTYNamespace ||
		!strings.HasPrefix(target.Pod, nodeTTYPodPrefix) ||
		target.Container != nodeTTYContainer {
		httpx.Fail(c, "PLUGIN_KUBERNETES_EXEC_COMMAND_INVALID", "invalid exec command", http.StatusBadRequest)
		return nil, false
	}
	wrapped := make([]string, 0, len(nodeTTYExecPrefix)+len(command))
	wrapped = append(wrapped, nodeTTYExecPrefix...)
	wrapped = append(wrapped, command...)
	return wrapped, true
}

func (h *Handler) startRecorder(ctx context.Context, target streamTarget, writer *sseWriter) *records.SessionRecorder {
	if h == nil || h.ctx.DB == nil {
		return nil
	}
	recorder, err := records.NewStore(h.ctx.DB).Start(ctx, records.Session{
		WorkspaceID: target.WorkspaceID.String(),
		ClusterID:   target.ClusterID.String(),
		Kind:        "exec",
		ObjectPath:  records.ObjectPathFromPod(target.Namespace, target.Pod, target.Container),
		StartedAt:   time.Now().UTC(),
	})
	if err != nil {
		_ = writer.Event("warning", "terminal recording unavailable")
		if h.ctx.Logger != nil {
			h.ctx.Logger.Warn("kubernetes exec recording start failed", zap.Error(err))
		}
		return nil
	}
	return recorder
}

func (h *Handler) closeRecorder(recorder *records.SessionRecorder) {
	if recorder == nil {
		return
	}
	if err := recorder.Close(context.Background(), time.Now().UTC()); err != nil && h != nil && h.ctx.Logger != nil {
		h.ctx.Logger.Warn("kubernetes exec recording close failed", zap.Error(err))
	}
}

func execOutputWriter(writer *sseWriter, recorder *records.SessionRecorder, event string, stream string) io.Writer {
	out := sseEventWriter{writer: writer, event: event}
	if recorder == nil {
		return out
	}
	switch stream {
	case "stdout":
		return io.MultiWriter(out, recorder.Stdout())
	case "stderr":
		return io.MultiWriter(out, recorder.Stderr())
	default:
		return out
	}
}

func recordExit(recorder *records.SessionRecorder, code int) {
	if recorder == nil {
		return
	}
	recorder.Event("exit", strconv.Itoa(code))
}

type sseEventWriter struct {
	writer *sseWriter
	event  string
}

func (w sseEventWriter) Write(p []byte) (int, error) {
	if err := w.writer.Event(w.event, string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

func commandFromQuery(c *gin.Context) ([]string, bool) {
	command := c.Request.URL.Query()["command"]
	if len(command) == 0 || len(command) > maxExecArgs {
		httpx.Fail(c, "PLUGIN_KUBERNETES_EXEC_COMMAND_INVALID", "invalid exec command", http.StatusBadRequest)
		return nil, false
	}
	total := 0
	out := make([]string, 0, len(command))
	for _, arg := range command {
		if arg == "" || len(arg) > maxExecArgBytes {
			httpx.Fail(c, "PLUGIN_KUBERNETES_EXEC_COMMAND_INVALID", "invalid exec command", http.StatusBadRequest)
			return nil, false
		}
		total += len(arg)
		if total > maxExecTotalBytes {
			httpx.Fail(c, "PLUGIN_KUBERNETES_EXEC_COMMAND_INVALID", "invalid exec command", http.StatusBadRequest)
			return nil, false
		}
		out = append(out, arg)
	}
	return out, true
}

func requireWrite(ctx context.Context, c *gin.Context, writer sparkplugin.WriteAuthorizer) (kernel.WorkspaceID, kernel.MemberID, bool) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	memberID, ok := streamMemberID(c)
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

func streamMemberID(c *gin.Context) (kernel.MemberID, bool) {
	return authn.MemberIDFromContext(c)
}

func recordExecAudit(ctx context.Context, audit sparkplugin.AuditLogger, target streamTarget, action, status string) {
	if audit == nil {
		return
	}
	detail := map[string]any{
		"cluster_id": target.ClusterID.String(),
		"namespace":  target.Namespace,
		"pod":        target.Pod,
	}
	if target.Container != "" {
		detail["container"] = target.Container
	}
	if status != "" {
		detail["status"] = status
	}
	_ = audit.Record(ctx, sparkplugin.AuditEvent{
		WorkspaceID: target.WorkspaceID.String(),
		MemberID:    target.MemberID.String(),
		Action:      action,
		Detail:      detail,
	})
}

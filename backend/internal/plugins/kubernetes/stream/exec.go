package stream

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
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
)

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
		Stdout:    sseEventWriter{writer: writer, event: "output"},
		Stderr:    sseEventWriter{writer: writer, event: "error"},
	}
	err = h.exec.StreamExec(c.Request.Context(), client, restCfg, req)
	if err == nil {
		status = "completed"
		_ = writer.Event("exit", map[string]int{"code": 0})
		return
	}
	if exitErr, ok := err.(clientgoexec.CodeExitError); ok {
		status = "failed"
		_ = writer.Event("exit", map[string]int{"code": exitErr.ExitStatus()})
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		status = "canceled"
		return
	}
	status = "failed"
	_ = writer.Event("error", "pod exec stream failed")
	_ = writer.Event("exit", map[string]int{"code": 1})
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

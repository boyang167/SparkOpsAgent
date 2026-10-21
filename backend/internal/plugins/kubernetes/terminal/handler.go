package terminal

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/records"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	clientgoexec "k8s.io/client-go/util/exec"
)

const (
	websocketReadTimeout = 70 * time.Second
	websocketPingPeriod  = 25 * time.Second
)

type TerminalExecutor interface {
	Stream(
		ctx context.Context,
		client kubernetes.Interface,
		restConfig *rest.Config,
		claims TicketClaims,
		session *Session,
		stdout io.Writer,
	) error
}

type KubernetesTerminalExecutor struct{}

func (KubernetesTerminalExecutor) Stream(
	ctx context.Context,
	client kubernetes.Interface,
	restConfig *rest.Config,
	claims TicketClaims,
	session *Session,
	stdout io.Writer,
) error {
	if client == nil || restConfig == nil {
		return errors.New("kubernetes terminal unavailable")
	}
	request := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(claims.Pod).
		Namespace(claims.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: claims.Container,
			Command:   ShellCommand(),
			Stdin:     true,
			Stdout:    true,
			Stderr:    false,
			TTY:       true,
		}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(restConfig, http.MethodPost, request.URL())
	if err != nil {
		return err
	}
	return executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             session,
		Stdout:            stdout,
		Tty:               true,
		TerminalSizeQueue: session,
	})
}

type Handler struct {
	ctx      sparkplugin.PluginContext
	tickets  *TicketStore
	executor TerminalExecutor
}

func NewHandler(ctx sparkplugin.PluginContext, tickets *TicketStore) *Handler {
	if tickets == nil {
		tickets = NewTicketStore(nil)
	}
	return &Handler{ctx: ctx, tickets: tickets, executor: KubernetesTerminalExecutor{}}
}

type createTicketRequest struct {
	Container string `json:"container"`
	Cols      uint16 `json:"cols"`
	Rows      uint16 `json:"rows"`
}

func (h *Handler) CreateTicket(c *gin.Context) {
	if h == nil || h.tickets == nil || h.ctx.Writer == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return
	}
	workspaceID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	allowed, err := h.ctx.Writer.CanWrite(c.Request.Context(), workspaceID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	if !allowed {
		httpx.Fail(c, "GOVERNANCE_API_ACCESS_DENIED", "governance admin permission required", http.StatusForbidden)
		return
	}

	clusterID, err := kernel.ParseClusterInfoID(c.Param("clusterId"))
	if err != nil {
		httpx.Fail(c, "PLUGIN_CLUSTER_INVALID", "invalid cluster id", http.StatusBadRequest)
		return
	}
	var request createTicketRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_TERMINAL_REQUEST_INVALID", "invalid terminal request", http.StatusBadRequest)
		return
	}
	namespace := c.Param("namespace")
	pod := c.Param("pod")
	if !validDNSLabel(namespace) || !validDNSSubdomain(pod) || !validDNSLabel(request.Container) {
		httpx.Fail(c, "PLUGIN_KUBERNETES_TERMINAL_TARGET_INVALID", "invalid terminal target", http.StatusBadRequest)
		return
	}
	if err := ValidateSize(request.Cols, request.Rows); err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_TERMINAL_SIZE_INVALID", "invalid terminal size", http.StatusBadRequest)
		return
	}
	if h.ctx.Clusters == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return
	}
	_, _, restConfig, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), workspaceID, clusterID)
	if err != nil || restConfig == nil || strings.TrimSpace(restConfig.Host) == "" {
		httpx.Fail(c, "PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE", "cluster credentials are incomplete", http.StatusServiceUnavailable)
		return
	}
	issued, err := h.tickets.Issue(TicketClaims{
		WorkspaceID: workspaceID,
		MemberID:    memberID,
		ClusterID:   clusterID,
		Namespace:   namespace,
		Pod:         pod,
		Container:   request.Container,
		Cols:        request.Cols,
		Rows:        request.Rows,
	})
	if err != nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_TERMINAL_TICKET_UNAVAILABLE", "terminal ticket unavailable", http.StatusTooManyRequests)
		return
	}
	httpx.OK(c, issued)
}

func (h *Handler) ServeWS(c *gin.Context) {
	if h == nil || h.tickets == nil {
		httpx.Fail(c, "PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable", http.StatusServiceUnavailable)
		return
	}
	if !AllowedOrigin(c.GetHeader("Origin"), c.Request.Host) {
		httpx.Fail(c, "PLUGIN_KUBERNETES_TERMINAL_ORIGIN_INVALID", "terminal origin not allowed", http.StatusForbidden)
		return
	}
	ticket := strings.TrimSpace(c.Query("ticket"))
	if _, ok := h.tickets.Peek(ticket); !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "invalid or expired terminal ticket", http.StatusUnauthorized)
		return
	}

	upgrader := websocket.Upgrader{CheckOrigin: func(request *http.Request) bool {
		return AllowedOrigin(request.Header.Get("Origin"), request.Host)
	}}
	connection, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer connection.Close()

	claims, ok := h.tickets.Consume(ticket)
	if !ok {
		writer := &socketWriter{connection: connection}
		_ = writer.JSON(ServerMessage{Type: "error", Message: "invalid or expired terminal ticket"})
		return
	}

	writer := &socketWriter{connection: connection}
	if err := writer.JSON(ServerMessage{Type: "open"}); err != nil {
		return
	}
	if h.ctx.Clusters == nil || h.executor == nil {
		_ = writer.JSON(ServerMessage{Type: "error", Message: "pod terminal unavailable"})
		return
	}
	client, _, restConfig, _, err := h.ctx.Clusters.ForWorkspace(c.Request.Context(), claims.WorkspaceID, claims.ClusterID)
	if err != nil || restConfig == nil || strings.TrimSpace(restConfig.Host) == "" {
		_ = writer.JSON(ServerMessage{Type: "error", Message: "pod terminal unavailable"})
		return
	}

	session := NewSession()
	defer session.Close()
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	session.SetOnWrite(func(message ServerMessage) {
		if writer.JSON(message) != nil {
			cancel()
		}
	})
	_ = session.HandleClientJSON(mustResizeMessage(claims.Cols, claims.Rows))

	connection.SetReadLimit(MaxInputBytes + 256)
	_ = connection.SetReadDeadline(time.Now().Add(websocketReadTimeout))
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(websocketReadTimeout))
	})

	go h.readClient(ctx, cancel, connection, session, writer)
	go h.pingClient(ctx, writer)

	recorder := h.startRecorder(ctx, claims)
	if recorder != nil {
		defer func() {
			_ = recorder.Close(context.Background(), time.Now().UTC())
		}()
	}
	recordTerminalAudit(ctx, h.ctx.Audit, claims, "kubernetes.exec.started", "")
	status := "failed"
	defer func() {
		recordTerminalAudit(context.Background(), h.ctx.Audit, claims, "kubernetes.exec.ended", status)
	}()

	stdout := io.Writer(session)
	if recorder != nil {
		stdout = io.MultiWriter(session, recorder.Stdout())
	}
	err = h.executor.Stream(ctx, client, restConfig, claims, session, stdout)
	session.Close()

	exitCode := 0
	switch {
	case err == nil:
		status = "completed"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status = "canceled"
		return
	default:
		exitCode = 1
		if exitError, ok := err.(clientgoexec.CodeExitError); ok {
			exitCode = exitError.ExitStatus()
		} else {
			_ = writer.JSON(ServerMessage{Type: "error", Message: "pod terminal session failed"})
		}
	}
	if recorder != nil {
		recorder.Event("exit", intString(exitCode))
	}
	_ = writer.JSON(ServerMessage{Type: "exit", Code: &exitCode})
}

func (h *Handler) readClient(
	ctx context.Context,
	cancel context.CancelFunc,
	connection *websocket.Conn,
	session *Session,
	writer *socketWriter,
) {
	defer cancel()
	for {
		messageType, raw, err := connection.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			_ = writer.JSON(ServerMessage{Type: "error", Message: "unsupported terminal message"})
			continue
		}
		if err := session.HandleClientJSON(raw); err != nil {
			_ = writer.JSON(ServerMessage{Type: "error", Message: "invalid terminal message"})
			continue
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func (h *Handler) pingClient(ctx context.Context, writer *socketWriter) {
	ticker := time.NewTicker(websocketPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if writer.Control(websocket.PingMessage, nil) != nil {
				return
			}
		}
	}
}

func (h *Handler) startRecorder(ctx context.Context, claims TicketClaims) *records.SessionRecorder {
	if h.ctx.DB == nil {
		return nil
	}
	recorder, err := records.NewStore(h.ctx.DB).Start(ctx, records.Session{
		WorkspaceID: claims.WorkspaceID.String(),
		ClusterID:   claims.ClusterID.String(),
		Kind:        "exec",
		ObjectPath:  records.ObjectPathFromPod(claims.Namespace, claims.Pod, claims.Container),
		StartedAt:   time.Now().UTC(),
	})
	if err != nil {
		return nil
	}
	return recorder
}

type socketWriter struct {
	connection *websocket.Conn
	mu         sync.Mutex
}

func (w *socketWriter) JSON(message ServerMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.connection.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	return w.connection.WriteJSON(message)
}

func (w *socketWriter) Control(messageType int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.connection.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	return w.connection.WriteControl(messageType, data, time.Now().Add(5*time.Second))
}

func AllowedOrigin(origin, host string) bool {
	if strings.TrimSpace(origin) == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(parsed.Host, host) {
		return true
	}
	return (host == "localhost:8080" && parsed.Host == "localhost:3000") ||
		(host == "127.0.0.1:8080" && parsed.Host == "127.0.0.1:3000")
}

func validDNSLabel(value string) bool {
	return len(validation.IsDNS1123Label(value)) == 0
}

func validDNSSubdomain(value string) bool {
	return len(validation.IsDNS1123Subdomain(value)) == 0
}

func mustResizeMessage(cols, rows uint16) []byte {
	return []byte(`{"type":"resize","cols":` + intString(int(cols)) + `,"rows":` + intString(int(rows)) + `}`)
}

func intString(value int) string {
	return strconv.Itoa(value)
}

func recordTerminalAudit(
	ctx context.Context,
	audit sparkplugin.AuditLogger,
	claims TicketClaims,
	action string,
	status string,
) {
	if audit == nil {
		return
	}
	detail := map[string]any{
		"cluster_id": claims.ClusterID.String(),
		"namespace":  claims.Namespace,
		"pod":        claims.Pod,
		"container":  claims.Container,
	}
	if status != "" {
		detail["status"] = status
	}
	_ = audit.Record(ctx, sparkplugin.AuditEvent{
		WorkspaceID: claims.WorkspaceID.String(),
		MemberID:    claims.MemberID.String(),
		Action:      action,
		Detail:      detail,
	})
}

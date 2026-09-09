package http

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	k8sterminal "github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/terminal"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	runtimeterminal "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/terminal"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	clientgoexec "k8s.io/client-go/util/exec"
)

const (
	projectTerminalReadTimeout = 70 * time.Second
	projectTerminalPingPeriod  = 25 * time.Second
)

type ProjectTerminalHandler struct {
	checker port.ProjectChecker
	lister  port.PodLister
	access  *k8s.ProjectClusterAccess
	tickets *runtimeterminal.TicketStore
}

func NewProjectTerminalHandler(
	checker port.ProjectChecker,
	lister port.PodLister,
	access *k8s.ProjectClusterAccess,
	tickets *runtimeterminal.TicketStore,
) *ProjectTerminalHandler {
	return &ProjectTerminalHandler{
		checker: checker, lister: lister, access: access, tickets: tickets,
	}
}

type createProjectTerminalTicketRequest struct {
	Container string `json:"container"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

func (h *ProjectTerminalHandler) CreateTicket(c *gin.Context) {
	if h == nil || h.tickets == nil || h.access == nil {
		httpx.Fail(c, "RUNTIME_TERMINAL_UNAVAILABLE", "project terminal unavailable", http.StatusServiceUnavailable)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	projectID, err := kernel.ParseProjectID(c.Param("projectId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
		return
	}
	podID, err := kernel.ParsePodID(c.Param("podId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid pod id", http.StatusBadRequest)
		return
	}
	var request createProjectTerminalTicketRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		httpx.Fail(c, "RUNTIME_TERMINAL_REQUEST_INVALID", "invalid terminal request", http.StatusBadRequest)
		return
	}
	if request.Cols <= 0 {
		request.Cols = 120
	}
	if request.Rows <= 0 {
		request.Rows = 36
	}
	if err := k8sterminal.ValidateSize(uint16(request.Cols), uint16(request.Rows)); err != nil {
		httpx.Fail(c, "RUNTIME_TERMINAL_SIZE_INVALID", "invalid terminal size", http.StatusBadRequest)
		return
	}
	inWs, err := h.checker.ProjectInWorkspace(c.Request.Context(), projectID, wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	if !inWs {
		httpx.Fail(c, "PROJECT_ACCESS_DENIED", "project access denied", http.StatusForbidden)
		return
	}
	projectName, err := h.checker.ProjectName(c.Request.Context(), projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	pod, err := h.lister.FindByID(c.Request.Context(), projectID, projectName, podID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	container := strings.TrimSpace(request.Container)
	if container == "" {
		container = strings.TrimSpace(pod.Container)
	}
	if container == "" {
		container = projectName
	}
	issued, err := h.tickets.Issue(runtimeterminal.TicketClaims{
		WorkspaceID: wsID,
		MemberID:    memberID,
		ProjectID:   projectID,
		PodID:       podID,
		Namespace:   pod.Namespace,
		Pod:         pod.Name,
		Container:   container,
		Cols:        uint16(request.Cols),
		Rows:        uint16(request.Rows),
	})
	if err != nil {
		httpx.Fail(c, "RUNTIME_TERMINAL_TICKET_UNAVAILABLE", "terminal ticket unavailable", http.StatusTooManyRequests)
		return
	}
	httpx.OK(c, issued)
}

func (h *ProjectTerminalHandler) ServeWS(c *gin.Context) {
	if h == nil || h.tickets == nil || h.access == nil {
		httpx.Fail(c, "RUNTIME_TERMINAL_UNAVAILABLE", "project terminal unavailable", http.StatusServiceUnavailable)
		return
	}
	if !allowedTerminalOrigin(c.GetHeader("Origin"), c.Request.Host) {
		httpx.Fail(c, "RUNTIME_TERMINAL_ORIGIN_INVALID", "terminal origin not allowed", http.StatusForbidden)
		return
	}
	ticket := strings.TrimSpace(c.Query("ticket"))
	if _, ok := h.tickets.Peek(ticket); !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "invalid or expired terminal ticket", http.StatusUnauthorized)
		return
	}

	upgrader := websocket.Upgrader{CheckOrigin: func(request *http.Request) bool {
		return allowedTerminalOrigin(request.Header.Get("Origin"), request.Host)
	}}
	connection, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer connection.Close()

	claims, ok := h.tickets.Consume(ticket)
	if !ok {
		writer := &projectTerminalWriter{connection: connection}
		_ = writer.JSON(k8sterminal.ServerMessage{Type: "error", Message: "invalid or expired terminal ticket"})
		return
	}

	writer := &projectTerminalWriter{connection: connection}
	if err := writer.JSON(k8sterminal.ServerMessage{Type: "open"}); err != nil {
		return
	}

	session, err := h.access.Open(c.Request.Context(), claims.WorkspaceID, claims.ProjectID)
	if err != nil || session == nil || session.RESTConfig == nil {
		_ = writer.JSON(k8sterminal.ServerMessage{Type: "error", Message: "pod terminal unavailable"})
		return
	}

	terminalSession := k8sterminal.NewSession()
	defer terminalSession.Close()
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	terminalSession.SetOnWrite(func(message k8sterminal.ServerMessage) {
		if writer.JSON(message) != nil {
			cancel()
		}
	})
	_ = terminalSession.HandleClientJSON(mustProjectResizeMessage(claims.Cols, claims.Rows))

	connection.SetReadLimit(k8sterminal.MaxInputBytes + 256)
	_ = connection.SetReadDeadline(time.Now().Add(projectTerminalReadTimeout))
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(projectTerminalReadTimeout))
	})

	go h.readProjectTerminalClient(ctx, cancel, connection, terminalSession, writer)
	go h.pingProjectTerminalClient(ctx, writer)

	executor := k8sterminal.KubernetesTerminalExecutor{}
	err = executor.Stream(ctx, session.Client, session.RESTConfig, k8sterminal.TicketClaims{
		Namespace: claims.Namespace,
		Pod:         claims.Pod,
		Container:   claims.Container,
		Cols:        claims.Cols,
		Rows:        claims.Rows,
	}, terminalSession, terminalSession)
	terminalSession.Close()

	exitCode := 0
	switch {
	case err == nil:
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return
	default:
		exitCode = 1
		if exitError, ok := err.(clientgoexec.CodeExitError); ok {
			exitCode = exitError.ExitStatus()
		} else {
			_ = writer.JSON(k8sterminal.ServerMessage{Type: "error", Message: "pod terminal session failed"})
		}
	}
	_ = writer.JSON(k8sterminal.ServerMessage{Type: "exit", Code: &exitCode})
}

func (h *ProjectTerminalHandler) readProjectTerminalClient(
	ctx context.Context,
	cancel context.CancelFunc,
	connection *websocket.Conn,
	session *k8sterminal.Session,
	writer *projectTerminalWriter,
) {
	defer cancel()
	for {
		messageType, raw, err := connection.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			_ = writer.JSON(k8sterminal.ServerMessage{Type: "error", Message: "unsupported terminal message"})
			continue
		}
		if err := session.HandleClientJSON(raw); err != nil {
			_ = writer.JSON(k8sterminal.ServerMessage{Type: "error", Message: "invalid terminal message"})
			continue
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func (h *ProjectTerminalHandler) pingProjectTerminalClient(ctx context.Context, writer *projectTerminalWriter) {
	ticker := time.NewTicker(projectTerminalPingPeriod)
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

type projectTerminalWriter struct {
	connection *websocket.Conn
	mu         sync.Mutex
}

func (w *projectTerminalWriter) JSON(message k8sterminal.ServerMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.connection.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	return w.connection.WriteJSON(message)
}

func (w *projectTerminalWriter) Control(messageType int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.connection.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	return w.connection.WriteControl(messageType, data, time.Now().Add(5*time.Second))
}

func allowedTerminalOrigin(origin, host string) bool {
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

func mustProjectResizeMessage(cols, rows uint16) []byte {
	return []byte(`{"type":"resize","cols":` + strconv.Itoa(int(cols)) + `,"rows":` + strconv.Itoa(int(rows)) + `}`)
}

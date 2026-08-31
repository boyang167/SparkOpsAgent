package terminal

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
	"github.com/gorilla/websocket"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestAllowedOrigin(t *testing.T) {
	require.True(t, AllowedOrigin("", "localhost:8080"))
	require.True(t, AllowedOrigin("http://localhost:3000", "localhost:8080"))
	require.True(t, AllowedOrigin("https://ops.example", "ops.example"))
	require.False(t, AllowedOrigin("https://evil.example", "ops.example"))
}

func TestCreateTicketRequiresWriteAndValidInput(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()

	for _, tc := range []struct {
		name   string
		writer sparkplugin.WriteAuthorizer
		body   string
		status int
	}{
		{name: "write denied", writer: fixedWriter(false), body: `{"container":"app","cols":80,"rows":24}`, status: http.StatusForbidden},
		{name: "invalid container", writer: fixedWriter(true), body: `{"container":"bad/name","cols":80,"rows":24}`, status: http.StatusBadRequest},
		{name: "invalid size", writer: fixedWriter(true), body: `{"container":"app","cols":1,"rows":24}`, status: http.StatusBadRequest},
		{name: "success", writer: fixedWriter(true), body: `{"container":"app","cols":80,"rows":24}`, status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewHandler(sparkplugin.PluginContext{
				Writer:   tc.writer,
				Clusters: terminalTestClusterFactory{},
			}, NewTicketStore(nil))
			engine := gin.New()
			engine.Use(func(c *gin.Context) {
				c.Set("workspace_id", wsID)
				c.Set("auth_member_id", memberID)
			})
			engine.POST("/clusters/:clusterId/namespaces/:namespace/pods/:pod/terminal-ticket", handler.CreateTicket)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(
				http.MethodPost,
				"/clusters/"+clusterID.String()+"/namespaces/default/pods/web-0/terminal-ticket",
				bytes.NewBufferString(tc.body),
			)
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(rec, req)
			require.Equal(t, tc.status, rec.Code)
			if tc.status == http.StatusOK {
				var envelope struct {
					Data IssuedTicket `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
				require.NotEmpty(t, envelope.Data.Ticket)
				require.Equal(t, WebsocketPath, envelope.Data.WebsocketPath)
			}
		})
	}
}

func TestCreateTicketRequiresClusterCredentials(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()
	clusterID := kernel.NewClusterInfoID()
	handler := NewHandler(sparkplugin.PluginContext{Writer: fixedWriter(true)}, NewTicketStore(nil))
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("workspace_id", wsID)
		c.Set("auth_member_id", memberID)
	})
	engine.POST("/clusters/:clusterId/namespaces/:namespace/pods/:pod/terminal-ticket", handler.CreateTicket)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/clusters/"+clusterID.String()+"/namespaces/default/pods/web-0/terminal-ticket",
		bytes.NewBufferString(`{"container":"app","cols":80,"rows":24}`),
	)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestServeWSConsumesTicketOnce(t *testing.T) {
	store := NewTicketStore(nil)
	issued, err := store.Issue(TicketClaims{
		WorkspaceID: kernel.NewWorkspaceID(),
		MemberID:    kernel.NewMemberID(),
		ClusterID:   kernel.NewClusterInfoID(),
		Namespace:   "default",
		Pod:         "web-0",
		Container:   "app",
		Cols:        80,
		Rows:        24,
	})
	require.NoError(t, err)
	handler := NewHandler(sparkplugin.PluginContext{}, store)
	engine := gin.New()
	engine.GET(WebsocketPath, handler.ServeWS)
	server := httptest.NewServer(engine)
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + WebsocketPath + "?ticket=" + issued.Ticket
	httpURL := server.URL + WebsocketPath + "?ticket=" + issued.Ticket
	response, err := http.Get(httpURL)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, response.StatusCode)
	require.NoError(t, response.Body.Close())

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	var message ServerMessage
	require.NoError(t, conn.ReadJSON(&message))
	require.Equal(t, "open", message.Type)
	require.NoError(t, conn.ReadJSON(&message))
	require.Equal(t, "error", message.Type)
	require.NoError(t, conn.Close())

	_, response, err = websocket.DefaultDialer.Dial(url, nil)
	require.Error(t, err)
	require.NotNil(t, response)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	require.NoError(t, response.Body.Close())
}

func TestServeWSBridgesInputAndCancelsOnDisconnect(t *testing.T) {
	store := NewTicketStore(nil)
	issued, err := store.Issue(TicketClaims{
		WorkspaceID: kernel.NewWorkspaceID(),
		MemberID:    kernel.NewMemberID(),
		ClusterID:   kernel.NewClusterInfoID(),
		Namespace:   "default",
		Pod:         "web-0",
		Container:   "app",
		Cols:        80,
		Rows:        24,
	})
	require.NoError(t, err)
	executor := &echoTerminalExecutor{canceled: make(chan struct{})}
	handler := NewHandler(sparkplugin.PluginContext{Clusters: terminalTestClusterFactory{}}, store)
	handler.executor = executor
	engine := gin.New()
	engine.GET(WebsocketPath, handler.ServeWS)
	server := httptest.NewServer(engine)
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + WebsocketPath + "?ticket=" + issued.Ticket
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	var message ServerMessage
	require.NoError(t, conn.ReadJSON(&message))
	require.Equal(t, "open", message.Type)
	require.NoError(t, conn.WriteJSON(ClientMessage{Type: "unknown"}))
	require.NoError(t, conn.ReadJSON(&message))
	require.Equal(t, "error", message.Type)
	require.NoError(t, conn.WriteJSON(ClientMessage{Type: "read", Data: "id\r"}))
	message = ServerMessage{}
	require.NoError(t, conn.ReadJSON(&message))
	require.Equal(t, ServerMessage{Type: "write", Data: "echo:id\r"}, message)
	require.NoError(t, conn.Close())

	select {
	case <-executor.canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("terminal executor was not canceled after websocket disconnect")
	}
}

type fixedWriter bool

func (w fixedWriter) CanWrite(context.Context, kernel.WorkspaceID, kernel.MemberID) (bool, error) {
	return bool(w), nil
}

type terminalTestClusterFactory struct{}

func (terminalTestClusterFactory) ForWorkspace(
	context.Context,
	kernel.WorkspaceID,
	kernel.ClusterInfoID,
) (kubernetes.Interface, dynamic.Interface, *rest.Config, *sparkplugin.ClusterMeta, error) {
	return fake.NewSimpleClientset(), nil, &rest.Config{Host: "https://kubernetes.example"}, nil, nil
}

func (terminalTestClusterFactory) ListMeta(
	context.Context,
	kernel.WorkspaceID,
) ([]sparkplugin.ClusterMeta, error) {
	return nil, nil
}

type echoTerminalExecutor struct {
	canceled chan struct{}
}

func (e *echoTerminalExecutor) Stream(
	ctx context.Context,
	_ kubernetes.Interface,
	_ *rest.Config,
	_ TicketClaims,
	session *Session,
	stdout io.Writer,
) error {
	input := make([]byte, 3)
	if _, err := io.ReadFull(session, input); err != nil {
		return err
	}
	if _, err := stdout.Write(append([]byte("echo:"), input...)); err != nil {
		return err
	}
	<-ctx.Done()
	close(e.canceled)
	return ctx.Err()
}

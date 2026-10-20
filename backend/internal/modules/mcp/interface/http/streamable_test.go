package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	governancedomain "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/application"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestHandleStreamableInitializeJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestMCPHandler(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mcp", bytes.NewReader([]byte(`{
		"jsonrpc":"2.0","id":1,"method":"initialize",
		"params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}
	}`)))
	c.Request.Header.Set("Accept", "application/json, text/event-stream")
	c.Request.Header.Set("Origin", "http://localhost:1234")
	seedMCPAuth(c)

	handler.HandleStreamable(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type=%q", ct)
	}
	var resp application.RPCResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil || resp.ID != float64(1) {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}

func TestHandleStreamableNotificationAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestMCPHandler(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mcp", bytes.NewReader([]byte(`{
		"jsonrpc":"2.0","method":"notifications/initialized"
	}`)))
	c.Request.Header.Set("Accept", "application/json, text/event-stream")
	seedMCPAuth(c)

	handler.HandleStreamable(c)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleStreamableGETMethodNotAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newTestMCPHandler(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/mcp", nil)
	c.Request.Header.Set("Accept", "text/event-stream")
	seedMCPAuth(c)

	handler.HandleStreamable(c)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestValidateMCPOriginRejectsRemote(t *testing.T) {
	if validateMCPOrigin("https://evil.example") {
		t.Fatal("expected remote origin rejected")
	}
}

func newTestMCPHandler(t *testing.T) *Handler {
	t.Helper()
	registry := domain.NewRegistry()
	registry.Register(domain.ToolSpec{
		Name:        "list_projects",
		Description: "List projects",
		InputSchema: domain.ObjectSchema(map[string]any{}),
		Permission:  "project:read",
		Tier:        domain.ToolTierDiscovery,
	}, func(context.Context, kernel.WorkspaceID, kernel.MemberID, map[string]any) (any, error) {
		return map[string]any{"items": []any{}}, nil
	})

	checker := streamableStubPermissionChecker{perms: map[string]bool{"project:read": true}}
	ents := streamableStubEntitlements{ents: entitlements.WorkspaceEntitlements{Mcp: true, Agent: true, Knowledge: true}}
	list := application.NewListToolsHandler(registry, checker, ents)
	rpc := application.NewRPCHandler(application.NewToolExecutor(registry, checker, ents), list)
	return NewHandler(rpc, list)
}

func seedMCPAuth(c *gin.Context) {
	wsID, _ := kernel.ParseWorkspaceID("c4d3a04c-1b10-4683-b3e0-f6cdbb86b7a9")
	memberID, _ := kernel.ParseMemberID("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	c.Set("auth_member_id", memberID)
	c.Set("workspace_id", wsID)
}

type streamableStubPermissionChecker struct {
	perms map[string]bool
}

func (s streamableStubPermissionChecker) RequirePermission(_ context.Context, _ kernel.WorkspaceID, _ kernel.MemberID, perm string) error {
	if s.perms[perm] {
		return nil
	}
	return governancedomain.ErrPermissionDenied
}

func (s streamableStubPermissionChecker) HasPermission(_ context.Context, _ kernel.WorkspaceID, _ kernel.MemberID, perm string) (bool, error) {
	return s.perms[perm], nil
}

type streamableStubEntitlements struct {
	ents entitlements.WorkspaceEntitlements
}

func (s streamableStubEntitlements) Resolve(_ context.Context, _ kernel.WorkspaceID) (entitlements.WorkspaceEntitlements, error) {
	return s.ents, nil
}

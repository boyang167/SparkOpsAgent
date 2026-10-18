package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestRPCHandlerToolsListUsesFilteredTools(t *testing.T) {
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

	entsProvider := stubEntitlementsProvider{ents: entitlements.WorkspaceEntitlements{
		Agent: true, Knowledge: true, Mcp: true,
	}}

	checker := stubPermissionChecker{perms: map[string]bool{"project:read": true}}
	list := NewListToolsHandler(registry, checker, entsProvider)
	rpc := NewRPCHandler(NewToolExecutor(registry, checker, entsProvider), list)

	wsID, err := kernel.ParseWorkspaceID("c4d3a04c-1b10-4683-b3e0-f6cdbb86b7a9")
	if err != nil {
		t.Fatal(err)
	}
	memberID, err := kernel.ParseMemberID("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
	if err != nil {
		t.Fatal(err)
	}

	resp := rpc.Handle(context.Background(), wsID, memberID, RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	})
	if resp.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", resp.Error)
	}
	raw, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Tools []struct {
			Name string `json:"name"`
			Tier string `json:"tier"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Tools) != 1 || payload.Tools[0].Name != "list_projects" {
		t.Fatalf("unexpected tools payload: %+v", payload.Tools)
	}
	if payload.Tools[0].Tier != "discovery" {
		t.Fatalf("expected tier discovery, got %q", payload.Tools[0].Tier)
	}
}

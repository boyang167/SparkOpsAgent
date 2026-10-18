package application

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type stubMCPPlugin struct{}

func (stubMCPPlugin) Register(*gin.Engine) {}

func (stubMCPPlugin) RegisterMCPTools(registrar sparkplugin.MCPToolRegistrar, _ sparkplugin.PluginContext) {
	registrar.Register("example", sparkplugin.MCPToolSpec{
		Name:        "example_plugin_tool",
		Description: "Example plugin tool",
		InputSchema: sparkplugin.ObjectSchema(map[string]any{}),
		Permission:  authz.PermProjectRead,
		Tier:        sparkplugin.MCPToolTierDiscovery,
	}, func(context.Context, kernel.WorkspaceID, kernel.MemberID, map[string]any) (any, error) {
		return map[string]any{"ok": true}, nil
	})
}

func TestRegisterPluginToolsFiltersByPluginEntitlement(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	sparkplugin.Register(stubMCPPlugin{})

	registry := domain.NewRegistry()
	RegisterPluginTools(registry, sparkplugin.PluginContext{})

	handler := NewListToolsHandler(registry, stubPermissionChecker{
		perms: map[string]bool{authz.PermProjectRead: true},
	}, stubEntitlementsProvider{
		ents: entitlements.WorkspaceEntitlements{},
	})
	tools, err := handler.Handle(context.Background(), mustWorkspaceID(t, "c4d3a04c-1b10-4683-b3e0-f6cdbb86b7a9"), mustMemberID(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		if tool.Name == "example_plugin_tool" {
			t.Fatal("expected plugin tool hidden when plugin disabled")
		}
	}

	handler = NewListToolsHandler(registry, stubPermissionChecker{
		perms: map[string]bool{authz.PermProjectRead: true},
	}, stubEntitlementsProvider{
		ents: entitlements.WorkspaceEntitlements{Plugins: []string{"example"}},
	})
	tools, err = handler.Handle(context.Background(), mustWorkspaceID(t, "c4d3a04c-1b10-4683-b3e0-f6cdbb86b7a9"), mustMemberID(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range tools {
		if tool.Name == "example_plugin_tool" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected plugin tool visible when plugin enabled")
	}
}

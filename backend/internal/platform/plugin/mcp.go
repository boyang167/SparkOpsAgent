package plugin

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MCPToolTier string

const (
	MCPToolTierDiscovery MCPToolTier = "discovery"
	MCPToolTierRead      MCPToolTier = "read"
	MCPToolTierAction    MCPToolTier = "action"
)

type MCPToolSpec struct {
	Name        string
	Description string
	InputSchema map[string]any
	Permission  string
	Modules     []string
	Tier        MCPToolTier
}

type MCPToolHandler func(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	args map[string]any,
) (any, error)

type MCPToolRegistrar interface {
	Register(pluginID string, spec MCPToolSpec, handler MCPToolHandler)
}

type MCPPlugin interface {
	RegisterMCPTools(registrar MCPToolRegistrar, ctx PluginContext)
}

func ObjectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func StringProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func IntegerProperty(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

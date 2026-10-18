package application

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type pluginToolRegistrar struct {
	registry *domain.Registry
}

func (r *pluginToolRegistrar) Register(pluginID string, spec sparkplugin.MCPToolSpec, handler sparkplugin.MCPToolHandler) {
	if pluginID == "" {
		panic("mcp plugin tool requires plugin id")
	}
	r.registry.Register(domain.ToolSpec{
		Name:        spec.Name,
		Description: spec.Description,
		InputSchema: spec.InputSchema,
		Permission:  spec.Permission,
		Modules:     append([]string(nil), spec.Modules...),
		PluginID:    pluginID,
		Tier:        domain.ToolTier(spec.Tier),
	}, wrapPluginHandler(handler))
}

func wrapPluginHandler(handler sparkplugin.MCPToolHandler) domain.ToolHandler {
	return func(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, args map[string]any) (any, error) {
		return handler(ctx, wsID, memberID, args)
	}
}

func RegisterPluginTools(registry *domain.Registry, pctx sparkplugin.PluginContext) {
	registrar := &pluginToolRegistrar{registry: registry}
	for _, p := range sparkplugin.Registered() {
		mp, ok := p.(sparkplugin.MCPPlugin)
		if !ok {
			continue
		}
		mp.RegisterMCPTools(registrar, pctx)
	}
}

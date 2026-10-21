package kubernetes

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func (p *plugin) RegisterMCPTools(registrar sparkplugin.MCPToolRegistrar, ctx sparkplugin.PluginContext) {
	registrar.Register("kubernetes", sparkplugin.MCPToolSpec{
		Name:        "list_kubernetes_clusters",
		Description: "List Kubernetes clusters authorized for the current workspace.",
		InputSchema: sparkplugin.ObjectSchema(map[string]any{}),
		Permission:  authz.PermProjectRead,
		Tier:        sparkplugin.MCPToolTierDiscovery,
	}, func(callCtx context.Context, wsID kernel.WorkspaceID, _ kernel.MemberID, _ map[string]any) (any, error) {
		if ctx.Clusters == nil {
			return nil, kernel.NewDomainError("PLUGIN_KUBERNETES_UNAVAILABLE", "kubernetes plugin context unavailable")
		}
		metas, err := ctx.Clusters.ListMeta(callCtx, wsID)
		if err != nil {
			return nil, err
		}
		items := make([]map[string]any, 0, len(metas))
		for _, meta := range metas {
			items = append(items, map[string]any{
				"id":             meta.ID,
				"name":           meta.Name,
				"server":         meta.Server,
				"prometheus_url": meta.PrometheusURL,
				"velaux_url":     meta.VelauxURL,
				"llm_model":      meta.LLMModel,
				"llm_base_url":   meta.LLMBaseURL,
				"has_prometheus_credentials": meta.PrometheusUser != "" || meta.PrometheusPassword != "",
				"has_velaux_credentials":     meta.VelauxUser != "" || meta.VelauxPassword != "",
				"has_llm_credentials":        meta.LLMAPIKey != "",
			})
		}
		return map[string]any{"items": items}, nil
	})
}

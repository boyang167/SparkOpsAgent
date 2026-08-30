package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type workspacePluginRepository interface {
	ListPlugins(ctx context.Context) ([]*model.PluginRecord, error)
	ListEnabledByWorkspace(ctx context.Context, workspaceID kernel.WorkspaceID) ([]*model.WorkspaceBinding, error)
}

type WorkspacePlugin struct {
	model.PluginRecord
	Enabled bool `json:"enabled"`
}

type ListWorkspacePluginsHandler struct {
	repo workspacePluginRepository
}

func NewListWorkspacePluginsHandler(repo workspacePluginRepository) *ListWorkspacePluginsHandler {
	return &ListWorkspacePluginsHandler{repo: repo}
}

func (h *ListWorkspacePluginsHandler) Handle(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
) ([]WorkspacePlugin, error) {
	plugins, err := h.repo.ListPlugins(ctx)
	if err != nil {
		return nil, err
	}
	bindings, err := h.repo.ListEnabledByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	enabled := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		if binding.Enabled {
			enabled[binding.PluginID] = struct{}{}
		}
	}
	items := make([]WorkspacePlugin, 0, len(plugins))
	for _, plugin := range plugins {
		if plugin.Status != model.PluginStatusDeployed {
			continue
		}
		_, isEnabled := enabled[plugin.ID]
		items = append(items, WorkspacePlugin{PluginRecord: *plugin, Enabled: isEnabled})
	}
	return items, nil
}

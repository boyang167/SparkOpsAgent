package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PluginRepository interface {
	SavePlugin(ctx context.Context, plugin *model.PluginRecord) error
	FindPlugin(ctx context.Context, id string) (*model.PluginRecord, error)
	ListPlugins(ctx context.Context) ([]*model.PluginRecord, error)
	DeletePlugin(ctx context.Context, id string) error
	SaveBinding(ctx context.Context, binding *model.WorkspaceBinding) error
	FindBinding(ctx context.Context, workspaceID kernel.WorkspaceID, pluginID string) (*model.WorkspaceBinding, error)
	ListBindingsByPlugin(ctx context.Context, pluginID string) ([]*model.WorkspaceBinding, error)
	ListEnabledByWorkspace(ctx context.Context, workspaceID kernel.WorkspaceID) ([]*model.WorkspaceBinding, error)
	DeleteBinding(ctx context.Context, workspaceID kernel.WorkspaceID, pluginID string) error
}

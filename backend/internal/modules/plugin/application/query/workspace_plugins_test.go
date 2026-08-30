package query_test

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type workspacePluginRepo struct {
	plugins  []*model.PluginRecord
	bindings []*model.WorkspaceBinding
}

func (r workspacePluginRepo) ListPlugins(context.Context) ([]*model.PluginRecord, error) {
	return r.plugins, nil
}

func (r workspacePluginRepo) ListEnabledByWorkspace(context.Context, kernel.WorkspaceID) ([]*model.WorkspaceBinding, error) {
	return r.bindings, nil
}

func TestListWorkspacePluginsReturnsOnlyDeployedWithEnabledFlag(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	handler := query.NewListWorkspacePluginsHandler(workspacePluginRepo{
		plugins: []*model.PluginRecord{
			{ID: "enabled", Status: model.PluginStatusDeployed},
			{ID: "disabled", Status: model.PluginStatusDeployed},
			{ID: "merged", Status: model.PluginStatusMerged},
		},
		bindings: []*model.WorkspaceBinding{
			{WorkspaceID: wsID, PluginID: "enabled", Enabled: true},
		},
	})

	items, err := handler.Handle(context.Background(), wsID)

	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "enabled", items[0].ID)
	require.True(t, items[0].Enabled)
	require.Equal(t, "disabled", items[1].ID)
	require.False(t, items[1].Enabled)
}

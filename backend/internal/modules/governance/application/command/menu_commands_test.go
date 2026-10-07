package command

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestMenuShouldBeRemoved_legacyClusterListByPath(t *testing.T) {
	menu := &model.Menu{
		WorkspaceID: kernel.NewWorkspaceID(),
		Path:        "/cluster/list",
		Title:       "集群管理",
	}
	require.True(t, menuShouldBeRemoved(menu))
}

func TestMenuShouldBeRemoved_kubernetesClustersMenuByTitle(t *testing.T) {
	menu := &model.Menu{
		WorkspaceID: kernel.NewWorkspaceID(),
		Path:        "/plugins/kubernetes/clusters",
		Title:       "集群管理",
	}
	require.False(t, menuShouldBeRemoved(menu))
}

func TestDefaultMenuGroups_titlesMatchRepairMap(t *testing.T) {
	for _, group := range defaultMenuGroups {
		require.Equal(t, group.Label, enterpriseMenuTitleByPath[group.ID], "group %s", group.ID)
		for _, item := range group.Items {
			require.Equal(t, item.Label, enterpriseMenuTitleByPath[item.Href], "menu %s", item.Href)
		}
	}
}

func TestDefaultMenuGroups_secretsLiveInSettingsBelowPlugins(t *testing.T) {
	var settings *defaultMenuGroup
	for i := range defaultMenuGroups {
		group := &defaultMenuGroups[i]
		for _, item := range group.Items {
			if item.Href == "/config/secrets" {
				settings = group
			}
		}
	}
	require.NotNil(t, settings)
	require.Equal(t, settingsGroupID, settings.ID)
	require.Greater(t, settings.Sort, 90)
}

func TestMenuShouldBeRemoved_legacySecretMenuByTitle(t *testing.T) {
	menu := &model.Menu{
		WorkspaceID: kernel.NewWorkspaceID(),
		Path:        "/config/old-secrets",
		Title:       "秘钥管理",
	}
	require.True(t, menuShouldBeRemoved(menu))
}

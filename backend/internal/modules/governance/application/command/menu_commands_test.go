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

func TestMenuShouldBeRemoved_legacySecretMenuByTitle(t *testing.T) {
	menu := &model.Menu{
		WorkspaceID: kernel.NewWorkspaceID(),
		Path:        "/config/old-secrets",
		Title:       "秘钥管理",
	}
	require.True(t, menuShouldBeRemoved(menu))
}

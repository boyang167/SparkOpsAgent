package persistence_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestBuildMenuTree_nestedChildren(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	parentID := "group-delivery"
	pid := parentID

	menus := []*model.Menu{
		{
			ID: parentID, WorkspaceID: wsID, Name: "delivery", Path: "delivery",
			Title: "交付运维", Sort: 1,
		},
		{
			ID: "child-builds", WorkspaceID: wsID, ParentID: &pid, Name: "builds",
			Path: "/delivery/builds", Title: "构建", Sort: 1,
		},
		{
			ID: "child-pods", WorkspaceID: wsID, ParentID: &pid, Name: "pods",
			Path: "/runtime/pods", Title: "Pod 运维", Sort: 2,
		},
	}

	tree := persistence.BuildMenuTree(menus)
	require.Len(t, tree, 1)
	require.Equal(t, "交付运维", tree[0].Title)
	require.Len(t, tree[0].Children, 2)
	require.Equal(t, "构建", tree[0].Children[0].Title)
	require.Equal(t, "Pod 运维", tree[0].Children[1].Title)
}

func TestBuildMenuTree_orphanBecomesRoot(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	missing := "missing-parent"

	menus := []*model.Menu{
		{
			ID: "orphan", WorkspaceID: wsID, ParentID: &missing, Name: "orphan",
			Path: "/orphan", Title: "孤儿菜单", Sort: 1,
		},
	}

	tree := persistence.BuildMenuTree(menus)
	require.Len(t, tree, 1)
	require.Equal(t, "孤儿菜单", tree[0].Title)
}

func TestIsSuperAdminOnlyPath_Plugins(t *testing.T) {
	require.True(t, persistence.IsSuperAdminOnlyPath("/platform/plugins"))
}

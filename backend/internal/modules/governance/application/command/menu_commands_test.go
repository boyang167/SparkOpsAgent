package command

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
	for _, item := range defaultDirectMenus {
		require.Equal(t, item.Label, enterpriseMenuTitleByPath[item.Href], "direct menu %s", item.Href)
	}
}

func TestDefaultMenuGroups_matchBusinessNavigation(t *testing.T) {
	require.Len(t, defaultMenuGroups, 3)

	require.Equal(t, "application", defaultMenuGroups[0].ID)
	require.Equal(t, "产品交付", defaultMenuGroups[0].Label)
	require.Equal(t, []string{
		"/publish/products",
		"/projects",
		"/publish/release-plans",
		"/products/workflows",
		"/cronjobs",
		"/templates",
	}, menuGroupPaths(defaultMenuGroups[0]))

	require.Equal(t, "ops-runtime", defaultMenuGroups[1].ID)
	require.Equal(t, "运行观测", defaultMenuGroups[1].Label)
	require.Equal(t, []string{
		"/runtime/pods",
		"/monitor/overview",
		"/monitor/log-viewer",
	}, menuGroupPaths(defaultMenuGroups[1]))

	require.Equal(t, settingsGroupID, defaultMenuGroups[2].ID)
	require.Equal(t, "配置与集成", defaultMenuGroups[2].Label)
	require.Equal(t, []string{
		"/config/secrets",
		"/config/webhooks",
		"/config/mcp",
	}, menuGroupPaths(defaultMenuGroups[2]))

	require.Equal(t, []defaultDirectMenu{
		{Href: "/knowledge", Label: "知识库", Sort: 95},
		{Href: "/files", Label: "文件中心", Icon: "Folder", Sort: 96},
	}, defaultDirectMenus)
}

func menuGroupPaths(group defaultMenuGroup) []string {
	paths := make([]string, 0, len(group.Items))
	for _, item := range group.Items {
		paths = append(paths, item.Href)
	}
	return paths
}

func TestRepairBusinessNavigation_preservesLeafIDsAndMovesMenus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS governance").Error)
	store := persistence.NewMenuStore(db)
	require.NoError(t, db.Exec(`
		CREATE TABLE governance.menus (
			id TEXT PRIMARY KEY, workspace_id TEXT, parent_id TEXT, name TEXT,
			path TEXT, title TEXT, icon TEXT, component TEXT, hidden BOOLEAN,
			sort INTEGER, keep_alive BOOLEAN, active_menu TEXT, close_tab BOOLEAN,
			is_basic BOOLEAN, transition_animation TEXT, created_at DATETIME,
			updated_at DATETIME, deleted_at DATETIME
		);
		CREATE TABLE governance.role_menus (role_id TEXT, menu_id TEXT);
		CREATE TABLE governance.menu_params (
			id TEXT PRIMARY KEY, menu_id TEXT, param_type TEXT, param_key TEXT,
			param_value TEXT, sort INTEGER, created_at DATETIME, updated_at DATETIME
		);
		CREATE TABLE governance.menu_buttons (
			id TEXT PRIMARY KEY, menu_id TEXT, name TEXT, code TEXT, remark TEXT,
			sort INTEGER, created_at DATETIME, updated_at DATETIME
		);
		CREATE TABLE governance.role_menu_buttons (role_id TEXT, button_id TEXT);
	`).Error)

	ctx := context.Background()
	workspaceID := kernel.NewWorkspaceID()
	groups := map[string]string{
		"application":       "group-application",
		"ops-delivery":      "group-delivery",
		"ops-runtime":       "group-runtime",
		"ops-observability": "group-observability",
		"ops-infra":         "group-infra",
	}
	for path, id := range groups {
		require.NoError(t, store.Save(ctx, &model.Menu{
			ID: id, WorkspaceID: workspaceID, Name: path, Path: path, Title: path,
		}))
	}

	legacyMenus := []struct {
		id       string
		path     string
		title    string
		parentID string
	}{
		{"menu-projects", "/projects", "项目", groups["application"]},
		{"menu-templates", "/templates", "模板", groups["application"]},
		{"menu-knowledge", "/knowledge", "知识库", groups["application"]},
		{"menu-files", "/files", "文件", groups["application"]},
		{"menu-workflows", "/products/workflows", "工作流", groups["ops-delivery"]},
		{"menu-release-plans", "/publish/release-plans", "发布计划", groups["ops-delivery"]},
		{"menu-pods", "/runtime/pods", "运行实例", groups["ops-runtime"]},
		{"menu-cronjobs", "/cronjobs", "定时任务", groups["ops-runtime"]},
		{"menu-monitor", "/monitor/overview", "监控", groups["ops-observability"]},
		{"menu-logs", "/monitor/log-viewer", "日志", groups["ops-observability"]},
		{"menu-secrets", "/config/secrets", "密钥", groups["ops-infra"]},
		{"menu-webhooks", "/config/webhooks", "外部集成", groups["ops-infra"]},
	}
	for _, item := range legacyMenus {
		parentID := item.parentID
		require.NoError(t, store.Save(ctx, &model.Menu{
			ID: item.id, WorkspaceID: workspaceID, ParentID: &parentID,
			Name: menuNameFromPath(item.path), Path: item.path, Title: item.title,
			Component: item.path,
		}))
	}
	hiddenProjects, err := store.FindByPath(ctx, workspaceID, "/projects")
	require.NoError(t, err)
	hiddenProjects.Hidden = true
	require.NoError(t, store.Save(ctx, hiddenProjects))
	require.NoError(t, db.Exec(
		"INSERT INTO governance.role_menus (role_id, menu_id) VALUES (?, ?), (?, ?)",
		"custom-role", "menu-secrets",
		"custom-role", "menu-projects",
	).Error)

	require.NoError(t, NewSeedWorkspaceMenusHandler(store).Handle(ctx, workspaceID))

	projects, err := store.FindByPath(ctx, workspaceID, "/projects")
	require.NoError(t, err)
	require.Equal(t, "menu-projects", projects.ID)
	require.NotNil(t, projects.ParentID)
	require.Equal(t, groups["application"], *projects.ParentID)
	require.Equal(t, 2, projects.Sort)
	require.True(t, projects.Hidden, "repair must preserve administrator visibility choices")

	knowledge, err := store.FindByPath(ctx, workspaceID, "/knowledge")
	require.NoError(t, err)
	require.Equal(t, "menu-knowledge", knowledge.ID)
	require.Nil(t, knowledge.ParentID)
	require.Equal(t, 95, knowledge.Sort)

	files, err := store.FindByPath(ctx, workspaceID, "/files")
	require.NoError(t, err)
	require.Equal(t, "menu-files", files.ID)
	require.Nil(t, files.ParentID)
	require.Equal(t, "文件中心", files.Title)
	require.Equal(t, "Folder", files.Icon)
	require.Equal(t, "/files", files.Component)
	require.Equal(t, 96, files.Sort)
	require.False(t, files.Hidden)

	oldDelivery, err := store.FindGroupMenu(ctx, workspaceID, "ops-delivery")
	require.NoError(t, err)
	require.True(t, oldDelivery.Hidden)
	oldObservability, err := store.FindGroupMenu(ctx, workspaceID, "ops-observability")
	require.NoError(t, err)
	require.True(t, oldObservability.Hidden)

	mcp, err := store.FindByPath(ctx, workspaceID, "/config/mcp")
	require.NoError(t, err)
	mcpRoleIDs, err := store.ListRoleIDs(ctx, mcp.ID)
	require.NoError(t, err)
	require.Empty(t, mcpRoleIDs, "MCP must receive its explicit default policy instead of sibling roles")

	products, err := store.FindByPath(ctx, workspaceID, "/publish/products")
	require.NoError(t, err)
	productRoleIDs, err := store.ListRoleIDs(ctx, products.ID)
	require.NoError(t, err)
	require.Empty(t, productRoleIDs, "new default menus must receive their explicit policy instead of sibling roles")
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

func TestRepairDuplicatePluginContainers_mergesChildrenAndHidesDuplicates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS governance").Error)
	store := persistence.NewMenuStore(db)
	require.NoError(t, db.Exec(`
		CREATE TABLE governance.menus (
			id TEXT PRIMARY KEY, workspace_id TEXT, parent_id TEXT, name TEXT,
			path TEXT, title TEXT, icon TEXT, component TEXT, hidden BOOLEAN,
			sort INTEGER, keep_alive BOOLEAN, active_menu TEXT, close_tab BOOLEAN,
			is_basic BOOLEAN, transition_animation TEXT, created_at DATETIME,
			updated_at DATETIME, deleted_at DATETIME
		);
	`).Error)

	ctx := context.Background()
	workspaceID := kernel.NewWorkspaceID()
	keeperID := "menu-plugins-keeper"
	dupID := "menu-plugins-dup"
	childID := "menu-k8s-clusters"
	require.NoError(t, store.Save(ctx, &model.Menu{
		ID: keeperID, WorkspaceID: workspaceID, Name: "plugins", Path: "/plugins", Title: "插件",
	}))
	require.NoError(t, store.Save(ctx, &model.Menu{
		ID: dupID, WorkspaceID: workspaceID, Name: "plugins", Path: "/plugins", Title: "插件",
	}))
	require.NoError(t, store.Save(ctx, &model.Menu{
		ID: childID, WorkspaceID: workspaceID, ParentID: &keeperID,
		Name: "plugin-k8s-clusters", Path: "/plugins/kubernetes/clusters", Title: "集群管理",
	}))

	require.NoError(t, NewSeedWorkspaceMenusHandler(store).repairDuplicatePluginContainers(ctx, workspaceID))

	keeperChildren, err := store.ListChildren(ctx, workspaceID, keeperID)
	require.NoError(t, err)
	require.Len(t, keeperChildren, 1)
	require.Equal(t, childID, keeperChildren[0].ID)

	dupChildren, err := store.ListChildren(ctx, workspaceID, dupID)
	require.NoError(t, err)
	require.Empty(t, dupChildren)

	dup, err := store.FindByID(ctx, workspaceID, dupID)
	require.NoError(t, err)
	require.True(t, dup.Hidden)
}

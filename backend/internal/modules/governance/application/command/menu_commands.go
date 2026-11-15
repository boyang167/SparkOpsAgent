package command

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SeedWorkspaceMenusHandler struct {
	menus *persistence.MenuStore
}

func NewSeedWorkspaceMenusHandler(menus *persistence.MenuStore) *SeedWorkspaceMenusHandler {
	return &SeedWorkspaceMenusHandler{menus: menus}
}

func (h *SeedWorkspaceMenusHandler) EnsureWorkspaceMenus(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
) error {
	return h.Handle(ctx, workspaceID)
}

func (h *SeedWorkspaceMenusHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID) error {
	count, err := h.menus.CountByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	if count == 0 {
		return h.seedAll(ctx, workspaceID)
	}
	return h.ensureDefaults(ctx, workspaceID)
}

func (h *SeedWorkspaceMenusHandler) seedAll(ctx context.Context, workspaceID kernel.WorkspaceID) error {
	sort := 1
	for _, group := range defaultMenuGroups {
		groupID := uuid.NewString()
		parentID := groupID
		groupSort := sort
		if group.Sort > 0 {
			groupSort = group.Sort
		}
		if err := h.menus.Save(ctx, &model.Menu{
			WorkspaceID: workspaceID,
			ID:          groupID,
			Name:        group.ID,
			Path:        group.ID,
			Title:       group.Label,
			Icon:        group.ID,
			Component:   "layout/routerHolder",
			Sort:        groupSort,
		}); err != nil {
			return err
		}
		sort++
		childSort := 1
		for _, item := range group.Items {
			name := menuNameFromPath(item.Href)
			pid := parentID
			if err := h.menus.Save(ctx, &model.Menu{
				WorkspaceID: workspaceID,
				ParentID:    &pid,
				Name:        name,
				Path:        item.Href,
				Title:       item.Label,
				Component:   item.Href,
				Sort:        childSort,
			}); err != nil {
				return err
			}
			childSort++
		}
	}
	for _, item := range defaultDirectMenus {
		if err := h.menus.Save(ctx, &model.Menu{
			WorkspaceID: workspaceID,
			ID:          uuid.NewString(),
			Name:        menuNameFromPath(item.Href),
			Path:        item.Href,
			Title:       item.Label,
			Icon:        item.Icon,
			Component:   item.Href,
			Sort:        item.Sort,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (h *SeedWorkspaceMenusHandler) ensureDefaults(ctx context.Context, workspaceID kernel.WorkspaceID) error {
	if err := h.repairLegacyMonitorMenus(ctx, workspaceID); err != nil {
		return err
	}
	if err := h.repairRemovedMenus(ctx, workspaceID); err != nil {
		return err
	}
	if err := h.repairBusinessNavigation(ctx, workspaceID); err != nil {
		return err
	}
	if err := h.repairDuplicatePluginContainers(ctx, workspaceID); err != nil {
		return err
	}
	return h.syncEnterpriseMenuTitles(ctx, workspaceID)
}

func (h *SeedWorkspaceMenusHandler) repairDuplicatePluginContainers(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
) error {
	menus, err := h.menus.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	containers := make([]*model.Menu, 0, 1)
	for _, menu := range menus {
		if menu.Path == "/plugins" && menu.ParentID == nil {
			containers = append(containers, menu)
		}
	}
	if len(containers) <= 1 {
		return nil
	}

	keeper := containers[0]
	keeperChildren := -1
	for _, candidate := range containers {
		children, listErr := h.menus.ListChildren(ctx, workspaceID, candidate.ID)
		if listErr != nil {
			return listErr
		}
		if len(children) > keeperChildren {
			keeper = candidate
			keeperChildren = len(children)
		}
	}

	for _, duplicate := range containers {
		if duplicate.ID == keeper.ID {
			continue
		}
		children, listErr := h.menus.ListChildren(ctx, workspaceID, duplicate.ID)
		if listErr != nil {
			return listErr
		}
		keeperID := keeper.ID
		for _, child := range children {
			child.ParentID = &keeperID
			if saveErr := h.menus.Save(ctx, child); saveErr != nil {
				return saveErr
			}
		}
		duplicate.Hidden = true
		if saveErr := h.menus.Save(ctx, duplicate); saveErr != nil {
			return saveErr
		}
	}
	return nil
}

func (h *SeedWorkspaceMenusHandler) repairLegacyMonitorMenus(ctx context.Context, workspaceID kernel.WorkspaceID) error {
	auditMenu, err := h.menus.FindByPath(ctx, workspaceID, "/monitor/audit")
	if err != nil {
		return nil
	}
	switch {
	case isOperationLogTitle(auditMenu.Title):
		auditMenu.Path = "/monitor/operation"
		auditMenu.Title = "操作日志"
		auditMenu.Name = "operation"
		auditMenu.Component = "/monitor/operation"
		return h.menus.Save(ctx, auditMenu)
	case auditMenu.Title == "组织审计":
		auditMenu.Path = "/monitor/org-audit"
		auditMenu.Name = "org-audit"
		auditMenu.Component = "/monitor/org-audit"
		return h.menus.Save(ctx, auditMenu)
	default:
		return nil
	}
}

var removedMenuPaths = []string{
	"/cluster/config",
	"/cluster/list",
	"/delivery/builds",
	"/publish/plans",
	"/publish/releases",
	"/monitor/statistics",
	"/market/dockerfiles",
	"/config/configmaps",
	"/config/storage",
	"/discovery/services",
	"/security/ingress",
	"/security/virtualservices",
	"/config/consul",
	"/delivery/pipelines",
	"/delivery/fleets",
	"/delivery/profiles",
}

var removedMenuTitles = []string{
	"秘钥管理",
	"服务连接配置",
	"能力目录",
	"部署统计",
	"构建管理",
	"版本计划",
	"发布管理",
	"Pipeline Studio",
	"Fleet 编辑器",
	"Profile 配置",
}

// misdeletedPluginMenuPaths lists plugin menus that were incorrectly soft-deleted by legacy title cleanup.
var misdeletedPluginMenuPaths = []string{
	"/plugins/kubernetes/clusters",
}

func normalizeMenuPathLocal(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}

func menuShouldBeRemoved(menu *model.Menu) bool {
	normalized := normalizeMenuPathLocal(menu.Path)
	for _, path := range removedMenuPaths {
		if normalized == normalizeMenuPathLocal(path) {
			return true
		}
	}
	title := strings.TrimSpace(menu.Title)
	for _, removedTitle := range removedMenuTitles {
		if title == removedTitle {
			// Plugin menus may reuse legacy titles (e.g. 集群管理 at /plugins/kubernetes/clusters).
			if strings.HasPrefix(normalized, "/plugins/") {
				continue
			}
			return true
		}
	}
	return false
}

func (h *SeedWorkspaceMenusHandler) RepairMisdeletedPluginMenusAllWorkspaces(ctx context.Context) error {
	workspaceIDs, err := h.menus.ListDistinctWorkspaceIDs(ctx)
	if err != nil {
		return err
	}
	for _, workspaceID := range workspaceIDs {
		for _, path := range misdeletedPluginMenuPaths {
			if _, err := h.menus.RestoreSoftDeletedByPath(ctx, workspaceID, path); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *SeedWorkspaceMenusHandler) RepairRemovedMenusAllWorkspaces(ctx context.Context) error {
	workspaceIDs, err := h.menus.ListDistinctWorkspaceIDs(ctx)
	if err != nil {
		return err
	}
	for _, workspaceID := range workspaceIDs {
		if err := h.repairRemovedMenus(ctx, workspaceID); err != nil {
			return err
		}
	}
	return h.menus.HardDeleteByPathsOrTitles(ctx, []string{
		"/delivery/pipelines",
		"/delivery/fleets",
		"/delivery/profiles",
	}, []string{
		"Pipeline Studio",
		"Fleet 编辑器",
		"Profile 配置",
	})
}

func (h *SeedWorkspaceMenusHandler) repairRemovedMenus(ctx context.Context, workspaceID kernel.WorkspaceID) error {
	menus, err := h.menus.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, menu := range menus {
		if !menuShouldBeRemoved(menu) {
			continue
		}
		if err := h.menus.SoftDeleteSubtree(ctx, workspaceID, menu.ID); err != nil {
			return err
		}
	}
	return nil
}

func isOperationLogTitle(title string) bool {
	switch strings.TrimSpace(title) {
	case "操作日志", "操作历史", "操作记录":
		return true
	default:
		return false
	}
}

func menuNameFromPath(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return "root"
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

type defaultMenuGroup struct {
	ID    string
	Label string
	// Sort overrides the position among root menus; zero keeps declaration order.
	Sort  int
	Items []defaultMenuItem
}

type defaultMenuItem struct {
	Href  string
	Label string
}

type defaultDirectMenu struct {
	Href  string
	Label string
	Icon  string
	Sort  int
}

const (
	settingsGroupID    = "ops-infra"
	settingsGroupTitle = "配置与集成"
	// settingsGroupSort keeps 配置与集成 below plugin roots, which are created with sort 90.
	settingsGroupSort = 100
)

var defaultMenuGroups = []defaultMenuGroup{
	{
		ID: "application", Label: "产品交付",
		Items: []defaultMenuItem{
			{"/publish/products", "产品"},
			{"/projects", "项目"},
			{"/publish/release-plans", "发布计划"},
			{"/products/workflows", "工作流"},
			{"/cronjobs", "定时构建"},
			{"/templates", "模板中心"},
		},
	},
	{
		ID: "ops-runtime", Label: "运行观测",
		Items: []defaultMenuItem{
			{"/runtime/pods", "运行实例"},
			{"/monitor/overview", "监控概览"},
			{"/monitor/log-viewer", "日志查询"},
		},
	},
	{
		ID: settingsGroupID, Label: settingsGroupTitle, Sort: settingsGroupSort,
		Items: []defaultMenuItem{
			{"/config/secrets", "密钥管理"},
			{"/config/webhooks", "外部集成"},
			{"/config/mcp", "MCP 集成"},
		},
	},
}

var defaultDirectMenus = []defaultDirectMenu{
	{Href: "/knowledge", Label: "知识库", Sort: 95},
	{Href: "/files", Label: "文件中心", Icon: "Folder", Sort: 96},
}

var legacyBusinessNavGroupIDs = []string{
	"delivery", "infra", "resource", "monitor", "permission", "platform", "settings",
}

// enterpriseMenuTitleByPath also covers group menus, whose path equals the group ID.
var enterpriseMenuTitleByPath = map[string]string{
	"application":            "产品交付",
	"ops-runtime":            "运行观测",
	settingsGroupID:          settingsGroupTitle,
	"/projects":              "项目",
	"/templates":             "模板中心",
	"/knowledge":             "知识库",
	"/files":                 "文件中心",
	"/files/data":            "数据面",
	"/files/control":         "控制面",
	"/files/governance":      "治理面",
	"/publish/products":      "产品",
	"/products/workflows":    "工作流",
	"/publish/release-plans": "发布计划",
	"/config/secrets":        "密钥管理",
	"/runtime/pods":          "运行实例",
	"/cronjobs":              "定时构建",
	"/monitor/overview":      "监控概览",
	"/monitor/log-viewer":    "日志查询",
	"/config/webhooks":       "外部集成",
	"/config/mcp":            "MCP 集成",
}

func (h *SeedWorkspaceMenusHandler) repairBusinessNavigation(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
) error {
	for index, group := range defaultMenuGroups {
		groupSort := index + 1
		if group.Sort > 0 {
			groupSort = group.Sort
		}
		parent, err := h.menus.FindGroupMenu(ctx, workspaceID, group.ID)
		if err != nil {
			parent, err = h.menus.FindByPath(ctx, workspaceID, group.ID)
		}
		if err != nil {
			parent = &model.Menu{
				WorkspaceID: workspaceID,
				Name:        group.ID,
				Path:        group.ID,
			}
		}
		parent.ParentID = nil
		parent.Name = group.ID
		parent.Path = group.ID
		parent.Title = group.Label
		parent.Icon = group.ID
		parent.Component = "layout/routerHolder"
		parent.Sort = groupSort
		if err := h.menus.Save(ctx, parent); err != nil {
			return err
		}

		for itemIndex, item := range group.Items {
			menu, findErr := h.menus.FindByPath(ctx, workspaceID, item.Href)
			if findErr != nil {
				menu = &model.Menu{
					WorkspaceID: workspaceID,
					Path:        item.Href,
				}
			}
			pid := parent.ID
			menu.ParentID = &pid
			menu.Name = menuNameFromPath(item.Href)
			menu.Title = item.Label
			menu.Component = item.Href
			menu.Sort = itemIndex + 1
			if err := h.menus.Save(ctx, menu); err != nil {
				return err
			}
		}
	}

	for _, item := range defaultDirectMenus {
		menu, err := h.menus.FindByPath(ctx, workspaceID, item.Href)
		if err != nil {
			menu = &model.Menu{
				WorkspaceID: workspaceID,
				Path:        item.Href,
			}
		}
		menu.ParentID = nil
		menu.Name = menuNameFromPath(item.Href)
		menu.Title = item.Label
		menu.Icon = item.Icon
		menu.Component = item.Href
		menu.Sort = item.Sort
		menu.Hidden = false
		if err := h.menus.Save(ctx, menu); err != nil {
			return err
		}
	}
	if err := h.repairFilesystemPlaneMenus(ctx, workspaceID); err != nil {
		return err
	}

	for _, groupID := range []string{"ops-delivery", "ops-observability"} {
		group, err := h.menus.FindGroupMenu(ctx, workspaceID, groupID)
		if err != nil {
			group, err = h.menus.FindByPath(ctx, workspaceID, groupID)
		}
		if err != nil {
			continue
		}
		children, err := h.menus.ListChildren(ctx, workspaceID, group.ID)
		if err != nil {
			return err
		}
		if len(children) == 0 {
			group.Hidden = true
			if err := h.menus.Save(ctx, group); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *SeedWorkspaceMenusHandler) repairFilesystemPlaneMenus(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
) error {
	parent, err := h.menus.FindByPath(ctx, workspaceID, "/files")
	if err != nil {
		return nil
	}
	parent.Icon = "Folder"
	parent.Component = "layout/routerHolder"
	parent.Title = "文件中心"
	parent.Hidden = false
	if err := h.menus.Save(ctx, parent); err != nil {
		return err
	}
	children := []struct {
		path      string
		title     string
		component string
		sort      int
	}{
		{"/files/data", "数据面", "/files/data/workspace", 1},
		{"/files/control", "控制面", "/files/control/providers", 2},
		{"/files/governance", "治理面", "/files/governance/audit", 3},
	}
	pid := parent.ID
	for _, child := range children {
		menu, findErr := h.menus.FindByPath(ctx, workspaceID, child.path)
		if findErr != nil {
			menu = &model.Menu{
				WorkspaceID: workspaceID,
				Path:        child.path,
			}
		}
		menu.ParentID = &pid
		menu.Name = menuNameFromPath(child.path)
		menu.Title = child.title
		menu.Component = child.component
		menu.Sort = child.sort
		menu.Hidden = false
		if err := h.menus.Save(ctx, menu); err != nil {
			return err
		}
	}
	return nil
}

// ensureSettingsGroup finds or creates the 集成 root group and pins it below plugin roots.
func (h *SeedWorkspaceMenusHandler) ensureSettingsGroup(ctx context.Context, workspaceID kernel.WorkspaceID) (*model.Menu, error) {
	group, err := h.menus.FindGroupMenu(ctx, workspaceID, settingsGroupID)
	if err != nil {
		group = &model.Menu{
			WorkspaceID: workspaceID,
			ID:          uuid.NewString(),
			Name:        settingsGroupID,
			Path:        settingsGroupID,
			Title:       settingsGroupTitle,
			Icon:        settingsGroupID,
			Component:   "layout/routerHolder",
			Sort:        settingsGroupSort,
		}
		if err := h.menus.Save(ctx, group); err != nil {
			return nil, err
		}
		return group, nil
	}
	if group.Sort == settingsGroupSort && group.Title == settingsGroupTitle {
		return group, nil
	}
	group.Sort = settingsGroupSort
	group.Title = settingsGroupTitle
	if err := h.menus.Save(ctx, group); err != nil {
		return nil, err
	}
	return group, nil
}

// repairSecretConfigMenu moves 「密钥」 from 交付 into 集成 as its first item.
func (h *SeedWorkspaceMenusHandler) repairSecretConfigMenu(ctx context.Context, workspaceID kernel.WorkspaceID) error {
	menu, err := h.menus.FindByPath(ctx, workspaceID, "/config/secrets")
	if err != nil {
		return nil
	}
	settingsGroup, err := h.ensureSettingsGroup(ctx, workspaceID)
	if err != nil {
		return err
	}
	changed := false
	if menu.Title != "密钥" {
		menu.Title = "密钥"
		menu.Name = menuNameFromPath(menu.Path)
		changed = true
	}
	if menu.ParentID == nil || *menu.ParentID != settingsGroup.ID {
		pid := settingsGroup.ID
		menu.ParentID = &pid
		menu.Sort = 0
		changed = true
	}
	if !changed {
		return nil
	}
	return h.menus.Save(ctx, menu)
}

// repairProductWorkflowsMenu ensures 「工作流」 sits under 交付, directly after 产品.
func (h *SeedWorkspaceMenusHandler) repairProductWorkflowsMenu(ctx context.Context, workspaceID kernel.WorkspaceID) error {
	const path = "/products/workflows"
	deliveryGroup, err := h.menus.FindGroupMenu(ctx, workspaceID, "ops-delivery")
	if err != nil {
		if deliveryGroup, err = h.menus.FindByPath(ctx, workspaceID, "ops-delivery"); err != nil {
			return nil
		}
	}

	products, err := h.menus.FindByPath(ctx, workspaceID, "/publish/products")
	desiredSort := 4
	if err == nil {
		desiredSort = products.Sort + 1
	}

	menu, err := h.menus.FindByPath(ctx, workspaceID, path)
	if err != nil {
		pid := deliveryGroup.ID
		menu = &model.Menu{
			WorkspaceID: workspaceID,
			ParentID:    &pid,
			Name:        menuNameFromPath(path),
			Path:        path,
			Title:       "工作流",
			Component:   path,
			Sort:        desiredSort,
		}
		if err := h.menus.Save(ctx, menu); err != nil {
			return err
		}
		return h.menus.AppendMenuToRolesWithSiblingBindings(ctx, workspaceID, deliveryGroup.ID, menu.ID)
	}

	changed := false
	if menu.Title != "工作流" {
		menu.Title = "工作流"
		menu.Name = menuNameFromPath(path)
		menu.Component = path
		changed = true
	}
	if menu.ParentID == nil || *menu.ParentID != deliveryGroup.ID {
		pid := deliveryGroup.ID
		menu.ParentID = &pid
		changed = true
	}
	if menu.Sort != desiredSort {
		menu.Sort = desiredSort
		changed = true
	}
	if !changed {
		return nil
	}
	return h.menus.Save(ctx, menu)
}

// repairReleasePlansMenu ensures 「发布计划」 sits under 交付, directly after 工作流.
func (h *SeedWorkspaceMenusHandler) repairReleasePlansMenu(ctx context.Context, workspaceID kernel.WorkspaceID) error {
	const path = "/publish/release-plans"
	deliveryGroup, err := h.menus.FindGroupMenu(ctx, workspaceID, "ops-delivery")
	if err != nil {
		if deliveryGroup, err = h.menus.FindByPath(ctx, workspaceID, "ops-delivery"); err != nil {
			return nil
		}
	}

	desiredSort := 5
	if workflows, err := h.menus.FindByPath(ctx, workspaceID, "/products/workflows"); err == nil {
		desiredSort = workflows.Sort + 1
	}

	menu, err := h.menus.FindByPath(ctx, workspaceID, path)
	if err != nil {
		pid := deliveryGroup.ID
		menu = &model.Menu{
			WorkspaceID: workspaceID,
			ParentID:    &pid,
			Name:        menuNameFromPath(path),
			Path:        path,
			Title:       "发布计划",
			Component:   path,
			Sort:        desiredSort,
		}
		if err := h.menus.Save(ctx, menu); err != nil {
			return err
		}
		return h.menus.AppendMenuToRolesWithSiblingBindings(ctx, workspaceID, deliveryGroup.ID, menu.ID)
	}

	changed := false
	if menu.Title != "发布计划" {
		menu.Title = "发布计划"
		menu.Name = menuNameFromPath(path)
		menu.Component = path
		changed = true
	}
	if menu.ParentID == nil || *menu.ParentID != deliveryGroup.ID {
		pid := deliveryGroup.ID
		menu.ParentID = &pid
		changed = true
	}
	if menu.Sort != desiredSort {
		menu.Sort = desiredSort
		changed = true
	}
	if !changed {
		return nil
	}
	return h.menus.Save(ctx, menu)
}

func (h *SeedWorkspaceMenusHandler) repairEnterpriseNavMenus(ctx context.Context, workspaceID kernel.WorkspaceID) error {
	if _, err := h.menus.FindGroupMenu(ctx, workspaceID, "application"); err != nil {
		for _, groupID := range legacyBusinessNavGroupIDs {
			parent, findErr := h.menus.FindGroupMenu(ctx, workspaceID, groupID)
			if findErr != nil {
				if parent, findErr = h.menus.FindByPath(ctx, workspaceID, groupID); findErr != nil {
					continue
				}
			}
			if err := h.menus.SoftDeleteSubtree(ctx, workspaceID, parent.ID); err != nil {
				return err
			}
		}
	}
	return h.syncEnterpriseMenuTitles(ctx, workspaceID)
}

func (h *SeedWorkspaceMenusHandler) syncEnterpriseMenuTitles(ctx context.Context, workspaceID kernel.WorkspaceID) error {
	menus, err := h.menus.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, menu := range menus {
		title, ok := enterpriseMenuTitleByPath[menu.Path]
		if !ok || menu.Title == title {
			continue
		}
		menu.Title = title
		menu.Name = menuNameFromPath(menu.Path)
		if err := h.menus.Save(ctx, menu); err != nil {
			return err
		}
	}
	return nil
}

type CreateMenuInput struct {
	ParentID            *string
	Name                string
	Path                string
	Title               string
	Icon                string
	Component           string
	Hidden              bool
	Sort                int
	KeepAlive           bool
	ActiveMenu          string
	CloseTab            bool
	IsBasic             bool
	TransitionAnimation string
	Params              []model.MenuParam
	Buttons             []model.MenuButton
}

type CreateMenuHandler struct {
	menus *persistence.MenuStore
}

func NewCreateMenuHandler(menus *persistence.MenuStore) *CreateMenuHandler {
	return &CreateMenuHandler{menus: menus}
}

func (h *CreateMenuHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, in CreateMenuInput) (*model.Menu, error) {
	name := strings.TrimSpace(in.Name)
	title := strings.TrimSpace(in.Title)
	path := strings.TrimSpace(in.Path)
	if name == "" || title == "" || path == "" {
		return nil, domain.ErrPermissionsRequired
	}
	if in.ParentID != nil && *in.ParentID != "" && *in.ParentID != "0" {
		if _, err := h.menus.FindByID(ctx, workspaceID, *in.ParentID); err != nil {
			return nil, err
		}
	} else {
		in.ParentID = nil
	}
	menu := &model.Menu{
		WorkspaceID:         workspaceID,
		ParentID:            in.ParentID,
		Name:                name,
		Path:                path,
		Title:               title,
		Icon:                strings.TrimSpace(in.Icon),
		Component:           strings.TrimSpace(in.Component),
		Hidden:              in.Hidden,
		Sort:                in.Sort,
		KeepAlive:           in.KeepAlive,
		ActiveMenu:          strings.TrimSpace(in.ActiveMenu),
		CloseTab:            in.CloseTab,
		IsBasic:             in.IsBasic,
		TransitionAnimation: strings.TrimSpace(in.TransitionAnimation),
	}
	if menu.TransitionAnimation == "" {
		menu.TransitionAnimation = "global"
	}
	if err := h.menus.Save(ctx, menu); err != nil {
		return nil, err
	}
	if err := h.menus.ReplaceParams(ctx, menu.ID, in.Params); err != nil {
		return nil, err
	}
	if err := h.menus.ReplaceButtons(ctx, menu.ID, in.Buttons); err != nil {
		return nil, err
	}
	menu.Params = in.Params
	menu.Buttons = in.Buttons
	return menu, nil
}

type UpdateMenuInput struct {
	ParentID            *string
	Name                string
	Path                string
	Title               string
	Icon                string
	Component           string
	Hidden              bool
	Sort                int
	KeepAlive           bool
	ActiveMenu          string
	CloseTab            bool
	IsBasic             bool
	TransitionAnimation string
	Params              []model.MenuParam
	Buttons             []model.MenuButton
}

type UpdateMenuHandler struct {
	menus *persistence.MenuStore
}

func NewUpdateMenuHandler(menus *persistence.MenuStore) *UpdateMenuHandler {
	return &UpdateMenuHandler{menus: menus}
}

func (h *UpdateMenuHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, menuID string, in UpdateMenuInput) error {
	menu, err := h.menus.FindByID(ctx, workspaceID, menuID)
	if err != nil {
		return err
	}
	if in.ParentID != nil && *in.ParentID != "" && *in.ParentID != "0" {
		if *in.ParentID == menuID {
			return domain.ErrInvalidParentRole
		}
		if _, err := h.menus.FindByID(ctx, workspaceID, *in.ParentID); err != nil {
			return err
		}
		menu.ParentID = in.ParentID
	} else {
		menu.ParentID = nil
	}
	menu.Name = strings.TrimSpace(in.Name)
	menu.Path = strings.TrimSpace(in.Path)
	menu.Title = strings.TrimSpace(in.Title)
	menu.Icon = strings.TrimSpace(in.Icon)
	menu.Component = strings.TrimSpace(in.Component)
	menu.Hidden = in.Hidden
	menu.Sort = in.Sort
	menu.KeepAlive = in.KeepAlive
	menu.ActiveMenu = strings.TrimSpace(in.ActiveMenu)
	menu.CloseTab = in.CloseTab
	menu.IsBasic = in.IsBasic
	menu.TransitionAnimation = strings.TrimSpace(in.TransitionAnimation)
	if menu.TransitionAnimation == "" {
		menu.TransitionAnimation = "global"
	}
	if menu.Name == "" || menu.Path == "" || menu.Title == "" {
		return domain.ErrPermissionsRequired
	}
	if err := h.menus.Save(ctx, menu); err != nil {
		return err
	}
	if err := h.menus.ReplaceParams(ctx, menuID, in.Params); err != nil {
		return err
	}
	return h.menus.ReplaceButtons(ctx, menuID, in.Buttons)
}

type DeleteMenuHandler struct {
	menus *persistence.MenuStore
}

func NewDeleteMenuHandler(menus *persistence.MenuStore) *DeleteMenuHandler {
	return &DeleteMenuHandler{menus: menus}
}

func (h *DeleteMenuHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, menuID string) error {
	return h.menus.SoftDeleteSubtree(ctx, workspaceID, menuID)
}

type SetMenuRolesHandler struct {
	menus    *persistence.MenuStore
	roleRepo repository.RoleRepository
}

func NewSetMenuRolesHandler(menus *persistence.MenuStore, roleRepo repository.RoleRepository) *SetMenuRolesHandler {
	return &SetMenuRolesHandler{menus: menus, roleRepo: roleRepo}
}

func (h *SetMenuRolesHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, menuID string, roleIDs []string) error {
	if _, err := h.menus.FindByID(ctx, workspaceID, menuID); err != nil {
		return err
	}
	valid := make([]string, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		id, err := kernel.ParseRoleID(roleID)
		if err != nil {
			return domain.ErrRoleNotFound
		}
		role, err := h.roleRepo.FindByID(ctx, id)
		if err != nil {
			return err
		}
		if role.WorkspaceID != workspaceID {
			return domain.ErrRoleNotFound
		}
		valid = append(valid, roleID)
	}
	return h.menus.SetRoleIDs(ctx, menuID, valid)
}

type SetRoleMenusHandler struct {
	menus    *persistence.MenuStore
	roleRepo repository.RoleRepository
}

func NewSetRoleMenusHandler(menus *persistence.MenuStore, roleRepo repository.RoleRepository) *SetRoleMenusHandler {
	return &SetRoleMenusHandler{menus: menus, roleRepo: roleRepo}
}

func (h *SetRoleMenusHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, roleID string, menuIDs []string) error {
	id, err := kernel.ParseRoleID(roleID)
	if err != nil {
		return domain.ErrRoleNotFound
	}
	role, err := h.roleRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if role.WorkspaceID != workspaceID {
		return domain.ErrRoleNotFound
	}
	valid := make([]string, 0, len(menuIDs))
	for _, menuID := range menuIDs {
		menuID = strings.TrimSpace(menuID)
		if menuID == "" {
			continue
		}
		valid = append(valid, menuID)
	}
	return h.menus.SetMenuIDsForRole(ctx, workspaceID, roleID, valid)
}

type SetRoleMenuButtonsHandler struct {
	menus    *persistence.MenuStore
	roleRepo repository.RoleRepository
}

func NewSetRoleMenuButtonsHandler(menus *persistence.MenuStore, roleRepo repository.RoleRepository) *SetRoleMenuButtonsHandler {
	return &SetRoleMenuButtonsHandler{menus: menus, roleRepo: roleRepo}
}

func (h *SetRoleMenuButtonsHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, roleID string, buttonIDs []string) error {
	id, err := kernel.ParseRoleID(roleID)
	if err != nil {
		return domain.ErrRoleNotFound
	}
	role, err := h.roleRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if role.WorkspaceID != workspaceID {
		return domain.ErrRoleNotFound
	}
	valid := make([]string, 0, len(buttonIDs))
	for _, buttonID := range buttonIDs {
		buttonID = strings.TrimSpace(buttonID)
		if buttonID == "" {
			continue
		}
		valid = append(valid, buttonID)
	}
	return h.menus.SetButtonIDsForRole(ctx, roleID, valid)
}

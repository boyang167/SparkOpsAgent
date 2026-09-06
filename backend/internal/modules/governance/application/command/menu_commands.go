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
		if err := h.menus.Save(ctx, &model.Menu{
			WorkspaceID: workspaceID,
			ID:          groupID,
			Name:        group.ID,
			Path:        group.ID,
			Title:       group.Label,
			Icon:        group.ID,
			Component:   "layout/routerHolder",
			Sort:        sort,
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
	return nil
}

func (h *SeedWorkspaceMenusHandler) ensureDefaults(ctx context.Context, workspaceID kernel.WorkspaceID) error {
	if err := h.repairLegacyMonitorMenus(ctx, workspaceID); err != nil {
		return err
	}
	if err := h.repairRemovedMenus(ctx, workspaceID); err != nil {
		return err
	}
	if err := h.repairEnterpriseNavMenus(ctx, workspaceID); err != nil {
		return err
	}
	if err := h.repairSecretConfigMenu(ctx, workspaceID); err != nil {
		return err
	}
	for _, group := range defaultMenuGroups {
		parent, err := h.menus.FindGroupMenu(ctx, workspaceID, group.ID)
		if err != nil {
			if parent, err = h.menus.FindByPath(ctx, workspaceID, group.ID); err != nil {
				continue
			}
		}
		children, err := h.menus.ListChildren(ctx, workspaceID, parent.ID)
		if err != nil {
			return err
		}
		maxSort := 0
		for _, child := range children {
			if child.Sort > maxSort {
				maxSort = child.Sort
			}
		}
		nextSort := maxSort + 1
		for _, item := range group.Items {
			existing, err := h.menus.FindByPath(ctx, workspaceID, item.Href)
			if err == nil {
				if existing.Title != item.Label {
					existing.Title = item.Label
					existing.Name = menuNameFromPath(item.Href)
					existing.Component = item.Href
					if err := h.menus.Save(ctx, existing); err != nil {
						return err
					}
				}
				continue
			}
			pid := parent.ID
			menu := &model.Menu{
				WorkspaceID: workspaceID,
				ParentID:    &pid,
				Name:        menuNameFromPath(item.Href),
				Path:        item.Href,
				Title:       item.Label,
				Component:   item.Href,
				Sort:        nextSort,
			}
			if err := h.menus.Save(ctx, menu); err != nil {
				return err
			}
			if err := h.menus.AppendMenuToRolesWithSiblingBindings(ctx, workspaceID, parent.ID, menu.ID); err != nil {
				return err
			}
			nextSort++
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
	"/market/dockerfiles",
	"/config/configmaps",
	"/config/storage",
	"/discovery/services",
	"/security/ingress",
	"/security/virtualservices",
	"/config/consul",
}

var removedMenuTitles = []string{
	"秘钥管理",
	"服务连接配置",
	"能力目录",
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
			return true
		}
	}
	return false
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
	return nil
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
	Items []struct {
		Href  string
		Label string
	}
}

var defaultMenuGroups = []defaultMenuGroup{
	{
		ID: "application", Label: "应用中心",
		Items: []struct{ Href, Label string }{
			{"/projects", "项目管理"},
			{"/templates", "模板管理"},
			{"/knowledge", "知识库"},
		},
	},
	{
		ID: "ops-delivery", Label: "持续交付",
		Items: []struct{ Href, Label string }{
			{"/delivery/builds", "构建管理"},
			{"/delivery/pipelines", "Pipeline Studio"},
			{"/delivery/fleets", "Fleet 编辑器"},
			{"/delivery/profiles", "Profile 配置"},
			{"/publish/products", "产品管理"},
			{"/publish/plans", "版本计划"},
			{"/publish/releases", "发布管理"},
			{"/config/secrets", "密钥配置"},
			{"/cluster/list", "集群管理"},
		},
	},
	{
		ID: "ops-runtime", Label: "运行管理",
		Items: []struct{ Href, Label string }{
			{"/runtime/pods", "运行实例"},
			{"/cronjobs", "定时任务"},
		},
	},
	{
		ID: "ops-observability", Label: "可观测性",
		Items: []struct{ Href, Label string }{
			{"/monitor/overview", "监控概览"},
			{"/monitor/log-viewer", "日志查询"},
			{"/monitor/statistics", "部署统计"},
		},
	},
	{
		ID: "ops-infra", Label: "基础设施",
		Items: []struct{ Href, Label string }{
			{"/config/webhooks", "外部集成"},
		},
	},
}

var legacyBusinessNavGroupIDs = []string{
	"delivery", "infra", "resource", "monitor", "permission", "platform", "settings",
}

var enterpriseMenuTitleByPath = map[string]string{
	"/projects":                 "项目管理",
	"/templates":                "模板管理",
	"/knowledge":                "知识库",
	"/delivery/builds":          "构建管理",
	"/delivery/pipelines":       "Pipeline Studio",
	"/delivery/fleets":          "Fleet 编辑器",
	"/delivery/profiles":        "Profile 配置",
	"/publish/products":         "产品管理",
	"/publish/plans":            "版本计划",
	"/publish/releases":         "发布管理",
	"/config/secrets":           "密钥配置",
	"/cluster/list":             "集群管理",
	"/runtime/pods":             "运行实例",
	"/cronjobs":                 "定时任务",
	"/monitor/overview":         "监控概览",
	"/monitor/log-viewer":       "日志查询",
	"/monitor/statistics":       "部署统计",
	"/config/webhooks": "外部集成",
}

func (h *SeedWorkspaceMenusHandler) repairSecretConfigMenu(ctx context.Context, workspaceID kernel.WorkspaceID) error {
	menu, err := h.menus.FindByPath(ctx, workspaceID, "/config/secrets")
	if err != nil {
		return nil
	}
	changed := false
	if menu.Title != "密钥配置" {
		menu.Title = "密钥配置"
		menu.Name = menuNameFromPath(menu.Path)
		changed = true
	}
	deliveryGroup, err := h.menus.FindGroupMenu(ctx, workspaceID, "ops-delivery")
	if err != nil {
		if deliveryGroup, err = h.menus.FindByPath(ctx, workspaceID, "ops-delivery"); err != nil {
			if !changed {
				return nil
			}
			return h.menus.Save(ctx, menu)
		}
	}
	if menu.ParentID == nil || *menu.ParentID != deliveryGroup.ID {
		pid := deliveryGroup.ID
		menu.ParentID = &pid
		menu.Sort = 5
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
		WorkspaceID: workspaceID,
		ParentID:    in.ParentID,
		Name:        name,
		Path:        path,
		Title:       title,
		Icon:        strings.TrimSpace(in.Icon),
		Component:   strings.TrimSpace(in.Component),
		Hidden:      in.Hidden,
		Sort:        in.Sort,
		KeepAlive:   in.KeepAlive,
		ActiveMenu:  strings.TrimSpace(in.ActiveMenu),
		CloseTab:    in.CloseTab,
		IsBasic:     in.IsBasic,
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

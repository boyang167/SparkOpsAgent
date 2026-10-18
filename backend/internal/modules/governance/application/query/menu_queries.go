package query

import (
	"context"
	"errors"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type MyMenuButtonItem struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Remark string `json:"remark"`
}

type MyMenuButtonsResult struct {
	MenuID     string             `json:"menu_id"`
	Path       string             `json:"path"`
	Configured bool               `json:"configured"`
	Items      []MyMenuButtonItem `json:"items"`
}

type workspaceMenuEnsurer interface {
	EnsureWorkspaceMenus(context.Context, kernel.WorkspaceID) error
}

type ListNavMenusHandler struct {
	menus       *persistence.MenuStore
	seed        workspaceMenuEnsurer
	checker     *Checker
	systemRoles authz.SystemRoleChecker
	db          *gorm.DB
}

func NewListNavMenusHandler(
	menus *persistence.MenuStore,
	seed workspaceMenuEnsurer,
	checker *Checker,
	systemRoles authz.SystemRoleChecker,
	db *gorm.DB,
) *ListNavMenusHandler {
	return &ListNavMenusHandler{menus: menus, seed: seed, checker: checker, systemRoles: systemRoles, db: db}
}

func (h *ListNavMenusHandler) Handle(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
	memberID kernel.MemberID,
) ([]model.MenuTreeNode, error) {
	if err := h.seed.EnsureWorkspaceMenus(ctx, workspaceID); err != nil {
		return nil, err
	}
	menus, err := h.menus.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	menuRoles, err := h.menus.ListRoleMenusByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	memberRoles, err := h.checker.ListRolesForMember(ctx, workspaceID, memberID)
	if err != nil {
		return nil, err
	}
	memberRoleIDs := make(map[string]struct{}, len(memberRoles))
	for _, role := range memberRoles {
		memberRoleIDs[role.ID.String()] = struct{}{}
	}
	isSuperAdmin, err := h.systemRoles.IsSuperAdmin(ctx, memberID)
	if err != nil {
		return nil, err
	}
	hasGovAdmin, err := h.checker.HasPermission(ctx, workspaceID, memberID, authz.PermGovernanceAdmin)
	if err != nil {
		return nil, err
	}
	ents, err := entitlements.ResolveCachedForWorkspace(ctx, h.db, workspaceID.String())
	if err != nil {
		return nil, err
	}
	filtered := persistence.FilterMenusForNav(menus, menuRoles, memberRoleIDs, isSuperAdmin, hasGovAdmin, ents)
	return persistence.BuildMenuTree(filtered), nil
}

type ListMenuTreeHandler struct {
	menus *persistence.MenuStore
	seed  workspaceMenuEnsurer
}

func NewListMenuTreeHandler(menus *persistence.MenuStore, seed workspaceMenuEnsurer) *ListMenuTreeHandler {
	return &ListMenuTreeHandler{menus: menus, seed: seed}
}

func (h *ListMenuTreeHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID) ([]model.MenuTreeNode, error) {
	if err := h.seed.EnsureWorkspaceMenus(ctx, workspaceID); err != nil {
		return nil, err
	}
	menus, err := h.menus.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return persistence.BuildMenuTree(menus), nil
}

type GetMenuHandler struct {
	menus *persistence.MenuStore
}

func NewGetMenuHandler(menus *persistence.MenuStore) *GetMenuHandler {
	return &GetMenuHandler{menus: menus}
}

func (h *GetMenuHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, menuID string) (*model.Menu, error) {
	return h.menus.FindDetailByID(ctx, workspaceID, menuID)
}

type GetMenuRolesHandler struct {
	menus *persistence.MenuStore
}

func NewGetMenuRolesHandler(menus *persistence.MenuStore) *GetMenuRolesHandler {
	return &GetMenuRolesHandler{menus: menus}
}

func (h *GetMenuRolesHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, menuID string) ([]string, error) {
	if _, err := h.menus.FindByID(ctx, workspaceID, menuID); err != nil {
		return nil, err
	}
	return h.menus.ListRoleIDs(ctx, menuID)
}

type GetRoleMenusHandler struct {
	menus    *persistence.MenuStore
	roleRepo repository.RoleRepository
}

func NewGetRoleMenusHandler(menus *persistence.MenuStore, roleRepo repository.RoleRepository) *GetRoleMenusHandler {
	return &GetRoleMenusHandler{menus: menus, roleRepo: roleRepo}
}

func (h *GetRoleMenusHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, roleID string) ([]string, error) {
	id, err := kernel.ParseRoleID(roleID)
	if err != nil {
		return nil, domain.ErrRoleNotFound
	}
	role, err := h.roleRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if role.WorkspaceID != workspaceID {
		return nil, domain.ErrRoleNotFound
	}
	return h.menus.ListMenuIDsByRole(ctx, workspaceID, roleID)
}

type WorkspaceMenuButtonView struct {
	ButtonID  string `json:"button_id"`
	MenuID    string `json:"menu_id"`
	MenuTitle string `json:"menu_title"`
	MenuPath  string `json:"menu_path"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Remark    string `json:"remark"`
}

type ListWorkspaceMenuButtonsHandler struct {
	menus *persistence.MenuStore
	seed  workspaceMenuEnsurer
}

func NewListWorkspaceMenuButtonsHandler(menus *persistence.MenuStore, seed workspaceMenuEnsurer) *ListWorkspaceMenuButtonsHandler {
	return &ListWorkspaceMenuButtonsHandler{menus: menus, seed: seed}
}

func (h *ListWorkspaceMenuButtonsHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID) ([]WorkspaceMenuButtonView, error) {
	if err := h.seed.EnsureWorkspaceMenus(ctx, workspaceID); err != nil {
		return nil, err
	}
	rows, err := h.menus.ListAllButtonsInWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkspaceMenuButtonView, 0, len(rows))
	for _, row := range rows {
		out = append(out, WorkspaceMenuButtonView{
			ButtonID: row.ButtonID, MenuID: row.MenuID, MenuTitle: row.MenuTitle,
			MenuPath: row.MenuPath, Name: row.Name, Code: row.Code, Remark: row.Remark,
		})
	}
	return out, nil
}

type GetRoleMenuButtonsHandler struct {
	menus    *persistence.MenuStore
	roleRepo repository.RoleRepository
}

func NewGetRoleMenuButtonsHandler(menus *persistence.MenuStore, roleRepo repository.RoleRepository) *GetRoleMenuButtonsHandler {
	return &GetRoleMenuButtonsHandler{menus: menus, roleRepo: roleRepo}
}

func (h *GetRoleMenuButtonsHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID, roleID string) ([]string, error) {
	id, err := kernel.ParseRoleID(roleID)
	if err != nil {
		return nil, domain.ErrRoleNotFound
	}
	role, err := h.roleRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if role.WorkspaceID != workspaceID {
		return nil, domain.ErrRoleNotFound
	}
	return h.menus.ListButtonIDsByRole(ctx, roleID)
}

type ListMyMenuButtonsHandler struct {
	menus       *persistence.MenuStore
	checker     *Checker
	systemRoles authz.SystemRoleChecker
}

func NewListMyMenuButtonsHandler(
	menus *persistence.MenuStore,
	checker *Checker,
	systemRoles authz.SystemRoleChecker,
) *ListMyMenuButtonsHandler {
	return &ListMyMenuButtonsHandler{menus: menus, checker: checker, systemRoles: systemRoles}
}

func (h *ListMyMenuButtonsHandler) Handle(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
	memberID kernel.MemberID,
	routePath string,
) (*MyMenuButtonsResult, error) {
	menu, err := h.menus.ResolveMenuByRoutePath(ctx, workspaceID, routePath)
	if errors.Is(err, domain.ErrMenuNotFound) {
		return &MyMenuButtonsResult{Path: routePath, Configured: false, Items: nil}, nil
	}
	if err != nil {
		return nil, err
	}

	buttons, err := h.menus.ListButtons(ctx, menu.ID)
	if err != nil {
		return nil, err
	}
	if len(buttons) == 0 {
		return &MyMenuButtonsResult{
			MenuID: menu.ID, Path: menu.Path, Configured: false, Items: nil,
		}, nil
	}

	isSuperAdmin, err := h.systemRoles.IsSuperAdmin(ctx, memberID)
	if err != nil {
		return nil, err
	}
	hasGovAdmin, err := h.checker.HasPermission(ctx, workspaceID, memberID, authz.PermGovernanceAdmin)
	if err != nil {
		return nil, err
	}

	memberRoles, err := h.checker.ListRolesForMember(ctx, workspaceID, memberID)
	if err != nil {
		return nil, err
	}
	memberRoleIDs := make(map[string]struct{}, len(memberRoles))
	for _, role := range memberRoles {
		memberRoleIDs[role.ID.String()] = struct{}{}
	}

	items := make([]MyMenuButtonItem, 0, len(buttons))
	for _, btn := range buttons {
		if isSuperAdmin || hasGovAdmin || buttonAllowedForMember(btn.RoleIDs, memberRoleIDs) {
			items = append(items, MyMenuButtonItem{
				Code: btn.Code, Name: btn.Name, Remark: btn.Remark,
			})
		}
	}

	return &MyMenuButtonsResult{
		MenuID: menu.ID, Path: menu.Path, Configured: true, Items: items,
	}, nil
}

func buttonAllowedForMember(roleIDs []string, memberRoleIDs map[string]struct{}) bool {
	if len(roleIDs) == 0 {
		return true
	}
	for _, roleID := range roleIDs {
		if _, ok := memberRoleIDs[roleID]; ok {
			return true
		}
	}
	return false
}

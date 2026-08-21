package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type menuModel struct {
	ID                  string `gorm:"column:id;primaryKey"`
	WorkspaceID         string
	ParentID            *string
	Name                string
	Path                string
	Title               string
	Icon                string
	Component           string
	Hidden              bool
	Sort                int
	KeepAlive           bool
	ActiveMenu          string `gorm:"column:active_menu"`
	CloseTab            bool   `gorm:"column:close_tab"`
	IsBasic             bool   `gorm:"column:is_basic"`
	TransitionAnimation string `gorm:"column:transition_animation"`
	postgres.AuditColumns
}

func (menuModel) TableName() string { return "governance.menus" }

type menuParamModel struct {
	ID         string `gorm:"column:id;primaryKey"`
	MenuID     string `gorm:"column:menu_id"`
	ParamType  string `gorm:"column:param_type"`
	ParamKey   string `gorm:"column:param_key"`
	ParamValue string `gorm:"column:param_value"`
	Sort       int
	postgres.AuditColumns
}

func (menuParamModel) TableName() string { return "governance.menu_params" }

type menuButtonModel struct {
	ID     string `gorm:"column:id;primaryKey"`
	MenuID string `gorm:"column:menu_id"`
	Name   string
	Code   string
	Remark string
	Sort   int
	postgres.AuditColumns
}

func (menuButtonModel) TableName() string { return "governance.menu_buttons" }

type roleMenuButtonModel struct {
	RoleID   string `gorm:"column:role_id;primaryKey"`
	ButtonID string `gorm:"column:button_id;primaryKey"`
}

func (roleMenuButtonModel) TableName() string { return "governance.role_menu_buttons" }

type roleMenuModel struct {
	RoleID string `gorm:"column:role_id;primaryKey"`
	MenuID string `gorm:"column:menu_id;primaryKey"`
}

func (roleMenuModel) TableName() string { return "governance.role_menus" }

type MenuStore struct {
	db *gorm.DB
}

func NewMenuStore(db *gorm.DB) *MenuStore {
	return &MenuStore{db: db}
}

func (s *MenuStore) AutoMigrate() error {
	if s.db.Dialector.Name() == "postgres" {
		if err := s.db.Exec("CREATE SCHEMA IF NOT EXISTS governance").Error; err != nil {
			return err
		}
	}
	return s.db.AutoMigrate(
		&menuModel{}, &roleMenuModel{}, &menuParamModel{}, &menuButtonModel{}, &roleMenuButtonModel{},
	)
}

func (s *MenuStore) CountByWorkspace(ctx context.Context, workspaceID kernel.WorkspaceID) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&menuModel{}).
		Where("workspace_id = ? AND deleted_at IS NULL", workspaceID.String()).
		Count(&count).Error
	return count, err
}

func (s *MenuStore) Save(ctx context.Context, menu *model.Menu) error {
	now := time.Now().UTC()
	if menu.ID == "" {
		menu.ID = uuid.NewString()
	}
	if menu.CreatedAt.IsZero() {
		menu.CreatedAt = now
	}
	menu.UpdatedAt = now
	row := menuModel{
		ID: menu.ID, WorkspaceID: menu.WorkspaceID.String(),
		ParentID: menu.ParentID, Name: menu.Name, Path: menu.Path,
		Title: menu.Title, Icon: menu.Icon, Component: menu.Component,
		Hidden: menu.Hidden, Sort: menu.Sort, KeepAlive: menu.KeepAlive,
		ActiveMenu: menu.ActiveMenu, CloseTab: menu.CloseTab, IsBasic: menu.IsBasic,
		TransitionAnimation: defaultTransition(menu.TransitionAnimation),
		AuditColumns:        postgres.AuditColumns{CreatedAt: menu.CreatedAt, UpdatedAt: menu.UpdatedAt},
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

func defaultTransition(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "global"
	}
	return v
}

func normalizeMenuPath(path string) string {
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

func (s *MenuStore) FindByPath(ctx context.Context, workspaceID kernel.WorkspaceID, path string) (*model.Menu, error) {
	path = normalizeMenuPath(path)
	if path == "" {
		return nil, domain.ErrMenuNotFound
	}
	var row menuModel
	err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND path = ? AND deleted_at IS NULL", workspaceID.String(), path).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrMenuNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToMenu(row), nil
}

func (s *MenuStore) ResolveMenuByRoutePath(ctx context.Context, workspaceID kernel.WorkspaceID, routePath string) (*model.Menu, error) {
	routePath = normalizeMenuPath(routePath)
	if routePath == "" {
		return nil, domain.ErrMenuNotFound
	}
	if menu, err := s.FindByPath(ctx, workspaceID, routePath); err == nil {
		return menu, nil
	} else if !errors.Is(err, domain.ErrMenuNotFound) {
		return nil, err
	}

	var row menuModel
	err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND active_menu = ? AND deleted_at IS NULL", workspaceID.String(), routePath).
		First(&row).Error
	if err == nil {
		return rowToMenu(row), nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	menus, err := s.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	var best *model.Menu
	bestLen := 0
	for _, menu := range menus {
		p := normalizeMenuPath(menu.Path)
		if p == "" || p == "/" {
			continue
		}
		if routePath == p || strings.HasPrefix(routePath, p+"/") {
			if len(p) > bestLen {
				bestLen = len(p)
				best = menu
			}
		}
	}
	if best != nil {
		return best, nil
	}
	return nil, domain.ErrMenuNotFound
}

func (s *MenuStore) FindByID(ctx context.Context, workspaceID kernel.WorkspaceID, menuID string) (*model.Menu, error) {
	var row menuModel
	err := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND deleted_at IS NULL", menuID, workspaceID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrMenuNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToMenu(row), nil
}

func (s *MenuStore) FindDetailByID(ctx context.Context, workspaceID kernel.WorkspaceID, menuID string) (*model.Menu, error) {
	menu, err := s.FindByID(ctx, workspaceID, menuID)
	if err != nil {
		return nil, err
	}
	params, err := s.ListParams(ctx, menuID)
	if err != nil {
		return nil, err
	}
	buttons, err := s.ListButtons(ctx, menuID)
	if err != nil {
		return nil, err
	}
	menu.Params = params
	menu.Buttons = buttons
	return menu, nil
}

func (s *MenuStore) ListParams(ctx context.Context, menuID string) ([]model.MenuParam, error) {
	var rows []menuParamModel
	if err := s.db.WithContext(ctx).Where("menu_id = ?", menuID).Order("sort ASC, created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.MenuParam, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.MenuParam{
			ID: row.ID, MenuID: row.MenuID, Type: row.ParamType,
			Key: row.ParamKey, Value: row.ParamValue, Sort: row.Sort,
		})
	}
	return out, nil
}

func (s *MenuStore) ListButtons(ctx context.Context, menuID string) ([]model.MenuButton, error) {
	var rows []menuButtonModel
	if err := s.db.WithContext(ctx).Where("menu_id = ?", menuID).Order("sort ASC, created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	buttonIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		buttonIDs = append(buttonIDs, row.ID)
	}
	roleMap, err := s.listButtonRoleIDs(ctx, buttonIDs)
	if err != nil {
		return nil, err
	}
	out := make([]model.MenuButton, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.MenuButton{
			ID: row.ID, MenuID: row.MenuID, Name: row.Name, Code: row.Code,
			Remark: row.Remark, Sort: row.Sort, RoleIDs: roleMap[row.ID],
		})
	}
	return out, nil
}

func (s *MenuStore) listButtonRoleIDs(ctx context.Context, buttonIDs []string) (map[string][]string, error) {
	out := make(map[string][]string)
	if len(buttonIDs) == 0 {
		return out, nil
	}
	var rows []roleMenuButtonModel
	if err := s.db.WithContext(ctx).Where("button_id IN ?", buttonIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ButtonID] = append(out[row.ButtonID], row.RoleID)
	}
	return out, nil
}

func (s *MenuStore) ReplaceParams(ctx context.Context, menuID string, params []model.MenuParam) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("menu_id = ?", menuID).Delete(&menuParamModel{}).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for i, param := range params {
			key := strings.TrimSpace(param.Key)
			if key == "" {
				continue
			}
			ptype := strings.TrimSpace(param.Type)
			if ptype == "" {
				ptype = "query"
			}
			id := param.ID
			if id == "" {
				id = uuid.NewString()
			}
			sort := param.Sort
			if sort == 0 {
				sort = i + 1
			}
			if err := tx.Create(&menuParamModel{
				ID: id, MenuID: menuID, ParamType: ptype, ParamKey: key,
				ParamValue: strings.TrimSpace(param.Value), Sort: sort,
				AuditColumns: postgres.AuditColumns{CreatedAt: now, UpdatedAt: now},
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *MenuStore) ReplaceButtons(ctx context.Context, menuID string, buttons []model.MenuButton) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var oldIDs []string
		if err := tx.Model(&menuButtonModel{}).Where("menu_id = ?", menuID).Pluck("id", &oldIDs).Error; err != nil {
			return err
		}
		if len(oldIDs) > 0 {
			if err := tx.Where("button_id IN ?", oldIDs).Delete(&roleMenuButtonModel{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("menu_id = ?", menuID).Delete(&menuButtonModel{}).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for i, btn := range buttons {
			code := strings.TrimSpace(btn.Code)
			name := strings.TrimSpace(btn.Name)
			if code == "" || name == "" {
				continue
			}
			id := btn.ID
			if id == "" {
				id = uuid.NewString()
			}
			sort := btn.Sort
			if sort == 0 {
				sort = i + 1
			}
			if err := tx.Create(&menuButtonModel{
				ID: id, MenuID: menuID, Name: name, Code: code,
				Remark: strings.TrimSpace(btn.Remark), Sort: sort,
				AuditColumns: postgres.AuditColumns{CreatedAt: now, UpdatedAt: now},
			}).Error; err != nil {
				return err
			}
			for _, roleID := range btn.RoleIDs {
				roleID = strings.TrimSpace(roleID)
				if roleID == "" {
					continue
				}
				if err := tx.Create(&roleMenuButtonModel{RoleID: roleID, ButtonID: id}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *MenuStore) FindGroupMenu(ctx context.Context, workspaceID kernel.WorkspaceID, groupKey string) (*model.Menu, error) {
	groupKey = strings.TrimSpace(groupKey)
	if groupKey == "" {
		return nil, domain.ErrMenuNotFound
	}
	var row menuModel
	err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND deleted_at IS NULL", workspaceID.String()).
		Where("(path = ? OR name = ?) AND (parent_id IS NULL OR parent_id = '' OR parent_id = '0')", groupKey, groupKey).
		Order("sort ASC, created_at ASC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrMenuNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToMenu(row), nil
}

func (s *MenuStore) ListChildren(ctx context.Context, workspaceID kernel.WorkspaceID, parentID string) ([]*model.Menu, error) {
	var rows []menuModel
	err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND parent_id = ? AND deleted_at IS NULL", workspaceID.String(), parentID).
		Order("sort ASC, created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Menu, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowToMenu(row))
	}
	return out, nil
}

// AppendMenuToRolesWithSiblingBindings grants a newly created menu to every role
// that already has at least one sibling menu under the same parent bound.
func (s *MenuStore) AppendMenuToRolesWithSiblingBindings(
	ctx context.Context,
	workspaceID kernel.WorkspaceID,
	parentID string,
	menuID string,
) error {
	children, err := s.ListChildren(ctx, workspaceID, parentID)
	if err != nil {
		return err
	}
	siblingIDs := make([]string, 0, len(children))
	for _, child := range children {
		if child.ID != menuID {
			siblingIDs = append(siblingIDs, child.ID)
		}
	}
	if len(siblingIDs) == 0 {
		return nil
	}

	var roleIDs []string
	err = s.db.WithContext(ctx).
		Model(&roleMenuModel{}).
		Where("menu_id IN ?", siblingIDs).
		Distinct().
		Pluck("role_id", &roleIDs).Error
	if err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}

	for _, roleID := range roleIDs {
		menuIDs, err := s.ListMenuIDsByRole(ctx, workspaceID, roleID)
		if err != nil {
			return err
		}
		hasNew := false
		for _, id := range menuIDs {
			if id == menuID {
				hasNew = true
				break
			}
		}
		if hasNew {
			continue
		}
		menuIDs = append(menuIDs, menuID)
		if err := s.SetMenuIDsForRole(ctx, workspaceID, roleID, menuIDs); err != nil {
			return err
		}
	}
	return nil
}

func (s *MenuStore) ListByWorkspace(ctx context.Context, workspaceID kernel.WorkspaceID) ([]*model.Menu, error) {
	var rows []menuModel
	err := s.db.WithContext(ctx).
		Where("workspace_id = ? AND deleted_at IS NULL", workspaceID.String()).
		Order("sort ASC, created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Menu, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowToMenu(row))
	}
	return out, nil
}

func (s *MenuStore) ListDistinctWorkspaceIDs(ctx context.Context) ([]kernel.WorkspaceID, error) {
	var raw []string
	err := s.db.WithContext(ctx).
		Model(&menuModel{}).
		Distinct("workspace_id").
		Pluck("workspace_id", &raw).Error
	if err != nil {
		return nil, err
	}
	out := make([]kernel.WorkspaceID, 0, len(raw))
	for _, id := range raw {
		wsID, err := kernel.ParseWorkspaceID(id)
		if err != nil {
			continue
		}
		out = append(out, wsID)
	}
	return out, nil
}

func (s *MenuStore) SoftDelete(ctx context.Context, workspaceID kernel.WorkspaceID, menuID string) error {
	res := s.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ? AND deleted_at IS NULL", menuID, workspaceID.String()).
		Delete(&menuModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrMenuNotFound
	}
	return nil
}

func (s *MenuStore) CountChildren(ctx context.Context, workspaceID kernel.WorkspaceID, menuID string) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&menuModel{}).
		Where("workspace_id = ? AND parent_id = ? AND deleted_at IS NULL", workspaceID.String(), menuID).
		Count(&count).Error
	return count, err
}

func (s *MenuStore) SoftDeleteSubtree(ctx context.Context, workspaceID kernel.WorkspaceID, menuID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []string
		if err := collectMenuSubtreeIDs(tx, workspaceID, menuID, &ids); err != nil {
			return err
		}
		if len(ids) == 0 {
			return domain.ErrMenuNotFound
		}
		if err := tx.Where("menu_id IN ?", ids).Delete(&roleMenuModel{}).Error; err != nil {
			return err
		}
		var buttonIDs []string
		if err := tx.Model(&menuButtonModel{}).Where("menu_id IN ?", ids).Pluck("id", &buttonIDs).Error; err != nil {
			return err
		}
		if len(buttonIDs) > 0 {
			if err := tx.Where("button_id IN ?", buttonIDs).Delete(&roleMenuButtonModel{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("menu_id IN ?", ids).Delete(&menuButtonModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("menu_id IN ?", ids).Delete(&menuParamModel{}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ? AND workspace_id = ?", ids, workspaceID.String()).Delete(&menuModel{}).Error
	})
}

func collectMenuSubtreeIDs(tx *gorm.DB, workspaceID kernel.WorkspaceID, menuID string, ids *[]string) error {
	var row menuModel
	if err := tx.Where("id = ? AND workspace_id = ? AND deleted_at IS NULL", menuID, workspaceID.String()).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrMenuNotFound
		}
		return err
	}
	*ids = append(*ids, menuID)
	var children []menuModel
	if err := tx.Where("workspace_id = ? AND parent_id = ? AND deleted_at IS NULL", workspaceID.String(), menuID).
		Find(&children).Error; err != nil {
		return err
	}
	for _, child := range children {
		if err := collectMenuSubtreeIDs(tx, workspaceID, child.ID, ids); err != nil {
			return err
		}
	}
	return nil
}

func (s *MenuStore) ListRoleIDs(ctx context.Context, menuID string) ([]string, error) {
	var rows []roleMenuModel
	if err := s.db.WithContext(ctx).Where("menu_id = ?", menuID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.RoleID)
	}
	return out, nil
}

func (s *MenuStore) SetRoleIDs(ctx context.Context, menuID string, roleIDs []string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("menu_id = ?", menuID).Delete(&roleMenuModel{}).Error; err != nil {
			return err
		}
		for _, roleID := range roleIDs {
			if roleID == "" {
				continue
			}
			if err := tx.Create(&roleMenuModel{RoleID: roleID, MenuID: menuID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *MenuStore) ListMenuIDsByRole(ctx context.Context, workspaceID kernel.WorkspaceID, roleID string) ([]string, error) {
	var menuIDs []string
	err := s.db.WithContext(ctx).
		Model(&roleMenuModel{}).
		Joins("JOIN governance.menus ON governance.menus.id = governance.role_menus.menu_id").
		Where("governance.role_menus.role_id = ?", roleID).
		Where("governance.menus.workspace_id = ? AND governance.menus.deleted_at IS NULL", workspaceID.String()).
		Pluck("governance.role_menus.menu_id", &menuIDs).Error
	if err != nil {
		return nil, err
	}
	return menuIDs, nil
}

func (s *MenuStore) SetMenuIDsForRole(ctx context.Context, workspaceID kernel.WorkspaceID, roleID string, menuIDs []string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var wsMenuIDs []string
		if err := tx.Table("governance.menus").
			Where("workspace_id = ? AND deleted_at IS NULL", workspaceID.String()).
			Pluck("id", &wsMenuIDs).Error; err != nil {
			return err
		}
		if len(wsMenuIDs) > 0 {
			if err := tx.Where("role_id = ? AND menu_id IN ?", roleID, wsMenuIDs).
				Delete(&roleMenuModel{}).Error; err != nil {
				return err
			}
		}
		for _, menuID := range menuIDs {
			menuID = strings.TrimSpace(menuID)
			if menuID == "" {
				continue
			}
			var count int64
			if err := tx.Table("governance.menus").
				Where("id = ? AND workspace_id = ? AND deleted_at IS NULL", menuID, workspaceID.String()).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return domain.ErrMenuNotFound
			}
			if err := tx.Create(&roleMenuModel{RoleID: roleID, MenuID: menuID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *MenuStore) ListRoleMenusByWorkspace(ctx context.Context, workspaceID kernel.WorkspaceID) (map[string][]string, error) {
	type roleMenuRow struct {
		MenuID string
		RoleID string
	}
	var rows []roleMenuRow
	err := s.db.WithContext(ctx).
		Model(&roleMenuModel{}).
		Select("governance.role_menus.menu_id, governance.role_menus.role_id").
		Joins("JOIN governance.menus ON governance.menus.id = governance.role_menus.menu_id").
		Where("governance.menus.workspace_id = ? AND governance.menus.deleted_at IS NULL", workspaceID.String()).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string][]string)
	for _, row := range rows {
		out[row.MenuID] = append(out[row.MenuID], row.RoleID)
	}
	return out, nil
}

func IsSuperAdminOnlyPath(path string) bool {
	return strings.HasPrefix(path, "/settings/platform") ||
		strings.HasPrefix(path, "/platform/") ||
		strings.HasPrefix(path, "/cluster/") ||
		path == "/monitor/platform-audit"
}

func IsAdminConsolePath(path string) bool {
	return strings.HasPrefix(path, "/admin/") || strings.HasPrefix(path, "/permission/")
}

func FilterMenusForNav(
	menus []*model.Menu,
	menuRoles map[string][]string,
	memberRoleIDs map[string]struct{},
	isSuperAdmin bool,
	hasGovAdmin bool,
	ents entitlements.WorkspaceEntitlements,
) []*model.Menu {
	byID := make(map[string]*model.Menu, len(menus))
	for _, menu := range menus {
		byID[menu.ID] = menu
	}

	direct := make(map[string]bool)
	for _, menu := range menus {
		if menu.Hidden {
			continue
		}
		if !ents.Allows(menu.Path) {
			continue
		}
		if IsSuperAdminOnlyPath(menu.Path) && !isSuperAdmin {
			continue
		}
		if IsAdminConsolePath(menu.Path) {
			continue
		}
		if strings.HasPrefix(menu.Path, "/permission/") && !hasGovAdmin {
			assigned := menuRoles[menu.ID]
			if len(assigned) == 0 || !hasRoleMatch(assigned, memberRoleIDs) {
				continue
			}
		} else if !menuVisibleForMember(menu.ID, menuRoles, memberRoleIDs) {
			continue
		}
		direct[menu.ID] = true
	}

	allowed := make(map[string]bool)
	for id := range direct {
		cur := id
		for cur != "" {
			allowed[cur] = true
			menu := byID[cur]
			if menu == nil || menu.ParentID == nil || *menu.ParentID == "" {
				break
			}
			cur = *menu.ParentID
		}
	}

	out := make([]*model.Menu, 0, len(allowed))
	for _, menu := range menus {
		if allowed[menu.ID] {
			out = append(out, menu)
		}
	}
	return out
}

func menuVisibleForMember(menuID string, menuRoles map[string][]string, memberRoleIDs map[string]struct{}) bool {
	assigned := menuRoles[menuID]
	if len(assigned) == 0 {
		return true
	}
	return hasRoleMatch(assigned, memberRoleIDs)
}

func hasRoleMatch(assigned []string, memberRoleIDs map[string]struct{}) bool {
	for _, roleID := range assigned {
		if _, ok := memberRoleIDs[roleID]; ok {
			return true
		}
	}
	return false
}

func rowToMenu(row menuModel) *model.Menu {
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	return &model.Menu{
		ID: row.ID, WorkspaceID: wsID, ParentID: row.ParentID,
		Name: row.Name, Path: row.Path, Title: row.Title, Icon: row.Icon,
		Component: row.Component, Hidden: row.Hidden, Sort: row.Sort,
		KeepAlive: row.KeepAlive, ActiveMenu: row.ActiveMenu, CloseTab: row.CloseTab,
		IsBasic: row.IsBasic, TransitionAnimation: row.TransitionAnimation,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func BuildMenuTree(menus []*model.Menu) []model.MenuTreeNode {
	if len(menus) == 0 {
		return nil
	}

	byMenu := make(map[string]*model.Menu, len(menus))
	childrenOf := make(map[string][]string)
	rootIDs := make([]string, 0)

	for _, menu := range menus {
		byMenu[menu.ID] = menu
	}

	for _, menu := range menus {
		if menu.ParentID == nil || *menu.ParentID == "" || *menu.ParentID == "0" {
			rootIDs = append(rootIDs, menu.ID)
			continue
		}
		parentID := *menu.ParentID
		if _, ok := byMenu[parentID]; !ok {
			rootIDs = append(rootIDs, menu.ID)
			continue
		}
		childrenOf[parentID] = append(childrenOf[parentID], menu.ID)
	}

	var build func(id string) model.MenuTreeNode
	build = func(id string) model.MenuTreeNode {
		menu := byMenu[id]
		node := model.MenuTreeNode{
			ID: menu.ID, ParentID: menu.ParentID, Name: menu.Name, Path: menu.Path,
			Title: menu.Title, Icon: menu.Icon, Component: menu.Component,
			Hidden: menu.Hidden, Sort: menu.Sort, KeepAlive: menu.KeepAlive,
			ActiveMenu: menu.ActiveMenu, CloseTab: menu.CloseTab, IsBasic: menu.IsBasic,
			TransitionAnimation: menu.TransitionAnimation,
		}
		for _, childID := range childrenOf[id] {
			node.Children = append(node.Children, build(childID))
		}
		sortMenuTreeNodes(node.Children)
		return node
	}

	roots := make([]model.MenuTreeNode, 0, len(rootIDs))
	for _, id := range rootIDs {
		roots = append(roots, build(id))
	}
	sortMenuTreeNodes(roots)
	return roots
}

func sortMenuTreeNodes(nodes []model.MenuTreeNode) {
	for i := range nodes {
		sortMenuTreeNodes(nodes[i].Children)
	}
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[j].Sort < nodes[i].Sort {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}
}

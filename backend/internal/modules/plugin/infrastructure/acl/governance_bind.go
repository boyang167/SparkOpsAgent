package acl

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/manifest"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GovernanceBinder struct {
	db *gorm.DB
}

func NewGovernanceBinder(db *gorm.DB) *GovernanceBinder {
	return &GovernanceBinder{db: db}
}

type menuRow struct {
	ID                  string     `gorm:"column:id;primaryKey"`
	WorkspaceID         string     `gorm:"column:workspace_id"`
	ParentID            *string    `gorm:"column:parent_id"`
	Name                string     `gorm:"column:name"`
	Path                string     `gorm:"column:path"`
	Title               string     `gorm:"column:title"`
	Icon                string     `gorm:"column:icon"`
	Component           string     `gorm:"column:component"`
	Hidden              bool       `gorm:"column:hidden"`
	Sort                int        `gorm:"column:sort"`
	KeepAlive           bool       `gorm:"column:keep_alive"`
	ActiveMenu          string     `gorm:"column:active_menu"`
	CloseTab            bool       `gorm:"column:close_tab"`
	IsBasic             bool       `gorm:"column:is_basic"`
	TransitionAnimation string     `gorm:"column:transition_animation"`
	CreatedAt           time.Time  `gorm:"column:created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at"`
	DeletedAt           *time.Time `gorm:"column:deleted_at"`
}

func (menuRow) TableName() string { return "governance.menus" }

type apiRow struct {
	ID          string    `gorm:"column:id;primaryKey"`
	Path        string    `gorm:"column:path"`
	Method      string    `gorm:"column:method"`
	APIGroup    string    `gorm:"column:api_group"`
	Description string    `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (apiRow) TableName() string { return "platform.apis" }

type rolePolicyRow struct {
	RoleID string `gorm:"column:role_id;primaryKey"`
	Path   string `gorm:"column:path;primaryKey"`
	Method string `gorm:"column:method;primaryKey"`
}

func (rolePolicyRow) TableName() string { return "governance.role_api_policies" }

type rolePermissionRow struct {
	ID          string `gorm:"column:id"`
	Permissions []byte `gorm:"column:permissions"`
}

func (b *GovernanceBinder) Bind(ctx context.Context, workspaceID kernel.WorkspaceID, spec *manifest.Manifest) error {
	db := b.db.WithContext(ctx)
	now := time.Now().UTC()
	parent := menuRow{
		ID: uuid.NewString(), WorkspaceID: workspaceID.String(),
		Name: "plugins", Path: "/plugins", Title: "插件",
		Sort: 90, TransitionAnimation: "global",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Where("workspace_id = ? AND path = ?", workspaceID.String(), "/plugins").
		Assign(map[string]any{
			"name": "plugins", "title": "插件", "updated_at": now, "deleted_at": nil,
		}).FirstOrCreate(&parent).Error; err != nil {
		return err
	}
	parentID := parent.ID
	for index, menu := range spec.Menus {
		row := menuRow{
			ID: uuid.NewString(), WorkspaceID: workspaceID.String(),
			ParentID: &parentID,
			Name:     fmt.Sprintf("plugin-%s-%d", spec.ID, index+1),
			Path:     menu.Path, Title: menu.Title, Component: menu.Component,
			Sort: index + 1, TransitionAnimation: "global",
			CreatedAt: now, UpdatedAt: now,
		}
		assign := map[string]any{
			"name": row.Name, "title": row.Title, "component": row.Component,
			"parent_id": parentID, "sort": row.Sort, "updated_at": now, "deleted_at": nil,
		}
		if err := db.Where("workspace_id = ? AND path = ?", workspaceID.String(), menu.Path).
			Assign(assign).FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}

	for _, api := range spec.APIs {
		row := apiRow{
			ID: uuid.NewString(), Path: api.Path, Method: strings.ToUpper(api.Method),
			APIGroup: "plugin:" + spec.ID, Description: spec.DisplayName,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "path"}, {Name: "method"}},
			DoUpdates: clause.Assignments(map[string]any{
				"api_group": row.APIGroup, "description": row.Description, "updated_at": now,
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}

	roleIDs, err := governanceAdminRoleIDs(db, workspaceID.String())
	if err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		for _, api := range spec.APIs {
			policy := rolePolicyRow{RoleID: roleID, Path: api.Path, Method: strings.ToUpper(api.Method)}
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&policy).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func governanceAdminRoleIDs(db *gorm.DB, workspaceID string) ([]string, error) {
	var rows []rolePermissionRow
	if err := db.Table("governance.roles").
		Select("id, permissions").
		Where("workspace_id = ?", workspaceID).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		var permissions []string
		if err := json.Unmarshal(row.Permissions, &permissions); err != nil {
			return nil, err
		}
		for _, permission := range permissions {
			if permission == authz.PermGovernanceAdmin {
				ids = append(ids, row.ID)
				break
			}
		}
	}
	return ids, nil
}

func (b *GovernanceBinder) Unbind(ctx context.Context, workspaceID kernel.WorkspaceID, pluginID string) error {
	db := b.db.WithContext(ctx)
	menuPrefix := "/plugins/" + pluginID
	if err := db.Where(
		"workspace_id = ? AND (path = ? OR path LIKE ?)",
		workspaceID.String(), menuPrefix, menuPrefix+"/%",
	).Delete(&menuRow{}).Error; err != nil {
		return err
	}

	apiPrefix := "/api/v1/plugins/" + pluginID
	roleIDs := db.Table("governance.roles").
		Select("id").
		Where("workspace_id = ?", workspaceID.String())
	return db.Where("role_id IN (?) AND (path = ? OR path LIKE ?)", roleIDs, apiPrefix, apiPrefix+"/%").
		Delete(&rolePolicyRow{}).Error
}

type UnitOfWork struct {
	db *gorm.DB
}

func NewUnitOfWork(db *gorm.DB) *UnitOfWork {
	return &UnitOfWork{db: db}
}

func (u *UnitOfWork) WithinTransaction(
	ctx context.Context,
	fn func(command.BindingRepository, command.WorkspaceBinder) error,
) error {
	return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(persistence.NewStore(tx), NewGovernanceBinder(tx))
	})
}

var _ command.WorkspaceBinder = (*GovernanceBinder)(nil)
var _ command.WorkspaceTransaction = (*UnitOfWork)(nil)

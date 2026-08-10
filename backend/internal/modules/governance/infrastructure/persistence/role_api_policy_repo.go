package persistence

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

type roleAPIPolicyModel struct {
	RoleID string `gorm:"column:role_id;primaryKey"`
	Path   string `gorm:"column:path;primaryKey"`
	Method string `gorm:"column:method;primaryKey"`
}

func (roleAPIPolicyModel) TableName() string { return "governance.role_api_policies" }

type RoleAPIPolicy struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

type RoleAPIPolicyStore struct{ db *gorm.DB }

func NewRoleAPIPolicyStore(db *gorm.DB) *RoleAPIPolicyStore {
	return &RoleAPIPolicyStore{db: db}
}

func (s *RoleAPIPolicyStore) AutoMigrate() error {
	if s.db.Dialector.Name() == "postgres" {
		if err := s.db.Exec("CREATE SCHEMA IF NOT EXISTS governance").Error; err != nil {
			return err
		}
	}
	return s.db.AutoMigrate(&roleAPIPolicyModel{})
}

func (s *RoleAPIPolicyStore) ListByRole(ctx context.Context, roleID string) ([]RoleAPIPolicy, error) {
	var rows []roleAPIPolicyModel
	if err := s.db.WithContext(ctx).Where("role_id = ?", roleID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]RoleAPIPolicy, 0, len(rows))
	for _, row := range rows {
		out = append(out, RoleAPIPolicy{Path: row.Path, Method: row.Method})
	}
	return out, nil
}

func (s *RoleAPIPolicyStore) ReplaceForRole(ctx context.Context, roleID string, policies []RoleAPIPolicy) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&roleAPIPolicyModel{}).Error; err != nil {
			return err
		}
		for _, p := range policies {
			path := strings.TrimSpace(p.Path)
			method := strings.ToUpper(strings.TrimSpace(p.Method))
			if path == "" || method == "" {
				continue
			}
			if err := tx.Create(&roleAPIPolicyModel{
				RoleID: roleID, Path: path, Method: method,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *RoleAPIPolicyStore) DeleteByRole(ctx context.Context, roleID string) error {
	return s.db.WithContext(ctx).Where("role_id = ?", roleID).Delete(&roleAPIPolicyModel{}).Error
}

func (s *RoleAPIPolicyStore) HasAnyForWorkspace(ctx context.Context, workspaceID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Table("governance.role_api_policies AS p").
		Joins("JOIN governance.roles r ON r.id = p.role_id").
		Where("r.workspace_id = ?", workspaceID).
		Count(&count).Error
	return count > 0, err
}

type rolePolicyRow struct {
	RoleID string
}

func (s *RoleAPIPolicyStore) ListRolesWithPolicies(ctx context.Context) ([]rolePolicyRow, error) {
	var roleIDs []string
	err := s.db.WithContext(ctx).
		Model(&roleAPIPolicyModel{}).
		Distinct("role_id").
		Pluck("role_id", &roleIDs).Error
	if err != nil {
		return nil, err
	}
	out := make([]rolePolicyRow, 0, len(roleIDs))
	for _, id := range roleIDs {
		out = append(out, rolePolicyRow{RoleID: id})
	}
	return out, nil
}

type bindingRow struct {
	RoleID   string `gorm:"column:role_id"`
	MemberID string `gorm:"column:member_id"`
}

func (s *RoleAPIPolicyStore) ListAllBindings(ctx context.Context) ([]bindingRow, error) {
	var rows []bindingRow
	err := s.db.WithContext(ctx).
		Table("governance.role_bindings").
		Select("role_id, member_id").
		Find(&rows).Error
	return rows, err
}

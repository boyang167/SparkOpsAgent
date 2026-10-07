package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SystemRoleStore struct {
	db *gorm.DB
}

func NewSystemRoleStore(db *gorm.DB) *SystemRoleStore {
	return &SystemRoleStore{db: db}
}

func (s *SystemRoleStore) AutoMigrate() error {
	if postgres.SQLMigrationsApplied(s.db) {
		return nil
	}
	if s.db.Dialector.Name() == "postgres" {
		if err := s.db.Exec("CREATE SCHEMA IF NOT EXISTS identity").Error; err != nil {
			return err
		}
	}
	return s.db.AutoMigrate(&systemRoleBindingModel{})
}

type systemRoleBindingModel struct {
	MemberID  string `gorm:"column:member_id;primaryKey"`
	Role      string `gorm:"column:role;not null"`
	GrantedAt time.Time
}

func (systemRoleBindingModel) TableName() string {
	return "identity.system_role_bindings"
}

func (s *SystemRoleStore) IsSuperAdmin(ctx context.Context, memberID kernel.MemberID) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&systemRoleBindingModel{}).
		Where("member_id = ? AND role = ?", memberID.String(), authz.SystemRoleSuperAdmin).
		Count(&count).Error
	return count > 0, err
}

func (s *SystemRoleStore) Grant(ctx context.Context, memberID kernel.MemberID, role string) error {
	row := systemRoleBindingModel{
		MemberID:  memberID.String(),
		Role:      role,
		GrantedAt: time.Now().UTC(),
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

func (s *SystemRoleStore) Revoke(ctx context.Context, memberID kernel.MemberID, role string) error {
	res := s.db.WithContext(ctx).
		Where("member_id = ? AND role = ?", memberID.String(), role).
		Delete(&systemRoleBindingModel{})
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (s *SystemRoleStore) CountByRole(ctx context.Context, role string) (int, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&systemRoleBindingModel{}).
		Where("role = ?", role).
		Count(&count).Error
	return int(count), err
}

func (s *SystemRoleStore) ListSuperAdminMemberIDs(ctx context.Context) (map[string]struct{}, error) {
	var rows []systemRoleBindingModel
	err := s.db.WithContext(ctx).Where("role = ?", authz.SystemRoleSuperAdmin).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		out[row.MemberID] = struct{}{}
	}
	return out, nil
}

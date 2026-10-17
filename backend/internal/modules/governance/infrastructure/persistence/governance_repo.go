package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func AutoMigrate(db *gorm.DB) error {
	if !postgres.ShouldRunAutoMigrate(db) {
		return nil
	}
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS governance").Error; err != nil {
			return err
		}
	}
	return db.AutoMigrate(&roleModel{}, &bindingModel{})
}

type roleModel struct {
	ID          string  `gorm:"column:id;primaryKey;type:uuid"`
	WorkspaceID string  `gorm:"type:uuid"`
	ParentID    *string `gorm:"type:uuid"`
	Name        string
	Permissions []byte `gorm:"type:jsonb;not null;default:'[]'"`
	postgres.AuditColumns
}

func (roleModel) TableName() string { return "governance.roles" }

type bindingModel struct {
	ID       string `gorm:"column:id;primaryKey;type:uuid"`
	RoleID   string `gorm:"type:uuid"`
	MemberID string `gorm:"type:uuid"`
	postgres.AuditColumns
}

func (bindingModel) TableName() string { return "governance.role_bindings" }

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) SaveRole(ctx context.Context, role *model.Role) error {
	raw, err := json.Marshal(role.Permissions)
	if err != nil {
		return err
	}
	var parentID *string
	if role.ParentID != nil {
		v := role.ParentID.String()
		parentID = &v
	}
	row := roleModel{
		ID: role.ID.String(), WorkspaceID: role.WorkspaceID.String(),
		ParentID: parentID, Name: role.Name, Permissions: raw,
		AuditColumns: postgres.AuditColumns{
			CreatedAt: role.CreatedAt,
			UpdatedAt: time.Now().UTC(),
		},
	}
	if row.CreatedAt.IsZero() {
		row.CreatedAt = row.UpdatedAt
	}
	return postgres.DB(ctx, s.db).Save(&row).Error
}

func (s *Store) FindRoleByID(ctx context.Context, id kernel.RoleID) (*model.Role, error) {
	var row roleModel
	err := postgres.DB(ctx, s.db).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrRoleNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToRole(row)
}

func (s *Store) ListRolesByWorkspace(ctx context.Context, workspaceID kernel.WorkspaceID) ([]*model.Role, error) {
	var rows []roleModel
	err := postgres.DB(ctx, s.db).Where("workspace_id = ?", workspaceID.String()).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Role, 0, len(rows))
	for _, row := range rows {
		role, err := rowToRole(row)
		if err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	return out, nil
}

func (s *Store) ListRolesByName(ctx context.Context, name string) ([]*model.Role, error) {
	var rows []roleModel
	err := postgres.DB(ctx, s.db).Where("name = ?", name).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Role, 0, len(rows))
	for _, row := range rows {
		role, err := rowToRole(row)
		if err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	return out, nil
}

func (s *Store) ExistsRoleByName(ctx context.Context, workspaceID kernel.WorkspaceID, name string) (bool, error) {
	var count int64
	err := postgres.DB(ctx, s.db).Model(&roleModel{}).
		Where("workspace_id = ? AND name = ?", workspaceID.String(), name).Count(&count).Error
	return count > 0, err
}

func (s *Store) SaveBinding(ctx context.Context, binding *model.RoleBinding) error {
	row := bindingModel{
		ID: binding.ID.String(), RoleID: binding.RoleID.String(),
		MemberID:     binding.MemberID.String(),
		AuditColumns: postgres.AuditColumns{CreatedAt: binding.CreatedAt, UpdatedAt: binding.CreatedAt},
	}
	return postgres.DB(ctx, s.db).Save(&row).Error
}

func (s *Store) ExistsBinding(ctx context.Context, roleID kernel.RoleID, memberID kernel.MemberID) (bool, error) {
	var count int64
	err := postgres.DB(ctx, s.db).Model(&bindingModel{}).
		Where("role_id = ? AND member_id = ?", roleID.String(), memberID.String()).Count(&count).Error
	return count > 0, err
}

func (s *Store) DeleteBinding(ctx context.Context, roleID kernel.RoleID, memberID kernel.MemberID) error {
	// Bindings are association rows: hard-delete so list queries and UNIQUE(role_id, member_id) stay consistent.
	result := postgres.DB(ctx, s.db).Unscoped().
		Where("role_id = ? AND member_id = ?", roleID.String(), memberID.String()).
		Delete(&bindingModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrBindingNotFound
	}
	return nil
}

func (s *Store) ListRolesForMember(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID) ([]*model.Role, error) {
	var rows []roleModel
	err := postgres.DB(ctx, s.db).Model(&roleModel{}).
		Joins("JOIN governance.role_bindings b ON b.role_id::uuid = governance.roles.id::uuid AND b.deleted_at IS NULL").
		Where("governance.roles.workspace_id = ? AND b.member_id = ?", workspaceID.String(), memberID.String()).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Role, 0, len(rows))
	for _, row := range rows {
		role, err := rowToRole(row)
		if err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	return out, nil
}

func (s *Store) DeleteRole(ctx context.Context, id kernel.RoleID) error {
	result := postgres.DB(ctx, s.db).Where("id = ?", id.String()).Delete(&roleModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrRoleNotFound
	}
	return nil
}

func (s *Store) DeleteBindingsByRoleID(ctx context.Context, roleID kernel.RoleID) error {
	return postgres.DB(ctx, s.db).Unscoped().
		Where("role_id = ?", roleID.String()).
		Delete(&bindingModel{}).Error
}

func (s *Store) CountRoleChildren(ctx context.Context, id kernel.RoleID) (int, error) {
	var count int64
	err := postgres.DB(ctx, s.db).Model(&roleModel{}).
		Where("parent_id = ?", id.String()).Count(&count).Error
	return int(count), err
}

func rowToRole(row roleModel) (*model.Role, error) {
	var perms []string
	if len(row.Permissions) > 0 {
		if err := json.Unmarshal(row.Permissions, &perms); err != nil {
			return nil, err
		}
	}
	roleID, _ := kernel.ParseRoleID(row.ID)
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	var parentID *kernel.RoleID
	if row.ParentID != nil && *row.ParentID != "" {
		if pid, err := kernel.ParseRoleID(*row.ParentID); err == nil {
			parentID = &pid
		}
	}
	return &model.Role{
		ID: roleID, WorkspaceID: wsID, ParentID: parentID, Name: row.Name,
		Permissions: perms, CreatedAt: row.CreatedAt,
	}, nil
}

type roleRepo struct{ store *Store }
type bindingRepo struct{ store *Store }

func NewRoleRepository(store *Store) repository.RoleRepository { return &roleRepo{store: store} }
func NewBindingRepository(store *Store) repository.BindingRepository {
	return &bindingRepo{store: store}
}

func (r *roleRepo) Save(ctx context.Context, role *model.Role) error {
	return r.store.SaveRole(ctx, role)
}
func (r *roleRepo) FindByID(ctx context.Context, id kernel.RoleID) (*model.Role, error) {
	return r.store.FindRoleByID(ctx, id)
}
func (r *roleRepo) ListByWorkspace(ctx context.Context, workspaceID kernel.WorkspaceID) ([]*model.Role, error) {
	return r.store.ListRolesByWorkspace(ctx, workspaceID)
}
func (r *roleRepo) ListByName(ctx context.Context, name string) ([]*model.Role, error) {
	return r.store.ListRolesByName(ctx, name)
}
func (r *roleRepo) ListWorkspaceIDs(ctx context.Context) ([]kernel.WorkspaceID, error) {
	var rows []string
	if err := postgres.DB(ctx, r.store.db).
		Table("tenant.workspaces").
		Where("deleted_at IS NULL").
		Pluck("id", &rows).Error; err != nil {
		return nil, err
	}
	out := make([]kernel.WorkspaceID, 0, len(rows))
	for _, raw := range rows {
		id, err := kernel.ParseWorkspaceID(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}
func (r *roleRepo) ExistsByName(ctx context.Context, workspaceID kernel.WorkspaceID, name string) (bool, error) {
	return r.store.ExistsRoleByName(ctx, workspaceID, name)
}
func (r *roleRepo) Delete(ctx context.Context, id kernel.RoleID) error {
	return r.store.DeleteRole(ctx, id)
}
func (r *roleRepo) CountChildren(ctx context.Context, id kernel.RoleID) (int, error) {
	return r.store.CountRoleChildren(ctx, id)
}

func (r *bindingRepo) Save(ctx context.Context, binding *model.RoleBinding) error {
	return r.store.SaveBinding(ctx, binding)
}
func (r *bindingRepo) ListRolesForMember(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID) ([]*model.Role, error) {
	return r.store.ListRolesForMember(ctx, workspaceID, memberID)
}
func (r *bindingRepo) Exists(ctx context.Context, roleID kernel.RoleID, memberID kernel.MemberID) (bool, error) {
	return r.store.ExistsBinding(ctx, roleID, memberID)
}
func (r *bindingRepo) Delete(ctx context.Context, roleID kernel.RoleID, memberID kernel.MemberID) error {
	return r.store.DeleteBinding(ctx, roleID, memberID)
}
func (r *bindingRepo) DeleteByRoleID(ctx context.Context, roleID kernel.RoleID) error {
	return r.store.DeleteBindingsByRoleID(ctx, roleID)
}

var (
	_ repository.RoleRepository    = (*roleRepo)(nil)
	_ repository.BindingRepository = (*bindingRepo)(nil)
)

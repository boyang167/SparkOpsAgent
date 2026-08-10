package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type RoleRepository interface {
	Save(ctx context.Context, role *model.Role) error
	FindByID(ctx context.Context, id kernel.RoleID) (*model.Role, error)
	ListByWorkspace(ctx context.Context, workspaceID kernel.WorkspaceID) ([]*model.Role, error)
	ExistsByName(ctx context.Context, workspaceID kernel.WorkspaceID, name string) (bool, error)
	Delete(ctx context.Context, id kernel.RoleID) error
	CountChildren(ctx context.Context, id kernel.RoleID) (int, error)
}

type BindingRepository interface {
	Save(ctx context.Context, binding *model.RoleBinding) error
	ListRolesForMember(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID) ([]*model.Role, error)
	Exists(ctx context.Context, roleID kernel.RoleID, memberID kernel.MemberID) (bool, error)
	Delete(ctx context.Context, roleID kernel.RoleID, memberID kernel.MemberID) error
	DeleteByRoleID(ctx context.Context, roleID kernel.RoleID) error
}

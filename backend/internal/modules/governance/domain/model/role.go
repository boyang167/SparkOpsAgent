package model

import (
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Role struct {
	ID           kernel.RoleID
	WorkspaceID  kernel.WorkspaceID
	ParentID     *kernel.RoleID
	Name         string
	Permissions  []string
	CreatedAt    time.Time
}

func NewRole(workspaceID kernel.WorkspaceID, name string, permissions []string, parentID *kernel.RoleID) *Role {
	perms := append([]string(nil), permissions...)
	return &Role{
		ID:          kernel.NewRoleID(),
		WorkspaceID: workspaceID,
		ParentID:    parentID,
		Name:        name,
		Permissions: perms,
		CreatedAt:   time.Now().UTC(),
	}
}

func (r *Role) HasPermission(perm string) bool {
	for _, p := range r.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

func (r *Role) UpdatePermissions(permissions []string) {
	r.Permissions = append([]string(nil), permissions...)
}

func (r *Role) IsProtected() bool {
	return r.Name == "admin"
}

type RoleBinding struct {
	ID       kernel.RoleBindingID
	RoleID   kernel.RoleID
	MemberID kernel.MemberID
	CreatedAt time.Time
}

func NewRoleBinding(roleID kernel.RoleID, memberID kernel.MemberID) *RoleBinding {
	return &RoleBinding{
		ID:       kernel.NewRoleBindingID(),
		RoleID:   roleID,
		MemberID: memberID,
		CreatedAt: time.Now().UTC(),
	}
}

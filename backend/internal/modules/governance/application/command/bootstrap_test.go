package command_test

import (
	"context"
	"sync"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type memoryRoleRepo struct {
	mu    sync.Mutex
	roles map[string]*model.Role
}

func newMemoryRoleRepo() *memoryRoleRepo {
	return &memoryRoleRepo{roles: make(map[string]*model.Role)}
}

func (r *memoryRoleRepo) Save(_ context.Context, role *model.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copyRole := *role
	r.roles[role.ID.String()] = &copyRole
	return nil
}

func (r *memoryRoleRepo) FindByID(_ context.Context, id kernel.RoleID) (*model.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	role, ok := r.roles[id.String()]
	if !ok {
		return nil, domain.ErrRoleNotFound
	}
	copyRole := *role
	return &copyRole, nil
}

func (r *memoryRoleRepo) ListByWorkspace(_ context.Context, workspaceID kernel.WorkspaceID) ([]*model.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.Role, 0)
	for _, role := range r.roles {
		if role.WorkspaceID == workspaceID {
			copyRole := *role
			out = append(out, &copyRole)
		}
	}
	return out, nil
}

func (r *memoryRoleRepo) ListByName(_ context.Context, name string) ([]*model.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.Role, 0)
	for _, role := range r.roles {
		if role.Name == name {
			copyRole := *role
			out = append(out, &copyRole)
		}
	}
	return out, nil
}

func (r *memoryRoleRepo) ExistsByName(_ context.Context, workspaceID kernel.WorkspaceID, name string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, role := range r.roles {
		if role.WorkspaceID == workspaceID && role.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *memoryRoleRepo) Delete(_ context.Context, id kernel.RoleID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.roles[id.String()]; !ok {
		return domain.ErrRoleNotFound
	}
	delete(r.roles, id.String())
	return nil
}

func (r *memoryRoleRepo) CountChildren(_ context.Context, id kernel.RoleID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, role := range r.roles {
		if role.ParentID != nil && role.ParentID.String() == id.String() {
			count++
		}
	}
	return count, nil
}

type memoryBindingRepo struct {
	mu       sync.Mutex
	bindings []*model.RoleBinding
}

func (r *memoryBindingRepo) Save(_ context.Context, binding *model.RoleBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings = append(r.bindings, binding)
	return nil
}

func (r *memoryBindingRepo) ListRolesForMember(_ context.Context, _ kernel.WorkspaceID, _ kernel.MemberID) ([]*model.Role, error) {
	return nil, nil
}

func (r *memoryBindingRepo) Exists(_ context.Context, _ kernel.RoleID, _ kernel.MemberID) (bool, error) {
	return false, nil
}

func (r *memoryBindingRepo) Delete(_ context.Context, _ kernel.RoleID, _ kernel.MemberID) error {
	return nil
}

func (r *memoryBindingRepo) DeleteByRoleID(_ context.Context, roleID kernel.RoleID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	filtered := r.bindings[:0]
	for _, b := range r.bindings {
		if b.RoleID != roleID {
			filtered = append(filtered, b)
		}
	}
	r.bindings = filtered
	return nil
}

func TestBootstrapWorkspaceCreatesAdminRole(t *testing.T) {
	ctx := context.Background()
	wsID := kernel.NewWorkspaceID()
	ownerID := kernel.NewMemberID()

	roles := newMemoryRoleRepo()
	bindings := &memoryBindingRepo{}
	handler := command.NewBootstrapWorkspaceHandler(roles, bindings, nil)

	err := handler.Handle(ctx, wsID, ownerID, authz.DefaultAdminPermissions)
	require.NoError(t, err)

	items, err := roles.ListByWorkspace(ctx, wsID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "admin", items[0].Name)
	require.Contains(t, items[0].Permissions, authz.PermGovernanceAdmin)

	err = handler.Handle(ctx, wsID, ownerID, authz.DefaultAdminPermissions)
	require.NoError(t, err)
	items, err = roles.ListByWorkspace(ctx, wsID)
	require.NoError(t, err)
	require.Len(t, items, 1)
}

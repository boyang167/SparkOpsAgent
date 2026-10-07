package query_test

import (
	"context"
	"sync"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
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
	perms := append([]string(nil), role.Permissions...)
	copyRole.Permissions = perms
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
	roles    repository.RoleRepository
}

func newMemoryBindingRepo(roles repository.RoleRepository) *memoryBindingRepo {
	return &memoryBindingRepo{roles: roles}
}

func (r *memoryBindingRepo) Save(_ context.Context, binding *model.RoleBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings = append(r.bindings, binding)
	return nil
}

func (r *memoryBindingRepo) ListRolesForMember(_ context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID) ([]*model.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.Role, 0)
	for _, b := range r.bindings {
		if b.MemberID != memberID {
			continue
		}
		role, err := r.roles.FindByID(context.Background(), b.RoleID)
		if err != nil {
			return nil, err
		}
		if role.WorkspaceID == workspaceID {
			out = append(out, role)
		}
	}
	return out, nil
}

func (r *memoryBindingRepo) Exists(_ context.Context, roleID kernel.RoleID, memberID kernel.MemberID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range r.bindings {
		if b.RoleID == roleID && b.MemberID == memberID {
			return true, nil
		}
	}
	return false, nil
}

func (r *memoryBindingRepo) Delete(_ context.Context, roleID kernel.RoleID, memberID kernel.MemberID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, b := range r.bindings {
		if b.RoleID == roleID && b.MemberID == memberID {
			r.bindings = append(r.bindings[:i], r.bindings[i+1:]...)
			return nil
		}
	}
	return domain.ErrBindingNotFound
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

func TestCheckerHasPermission(t *testing.T) {
	ctx := context.Background()
	wsID := kernel.NewWorkspaceID()
	memberID := kernel.NewMemberID()

	roles := newMemoryRoleRepo()
	bindings := newMemoryBindingRepo(roles)
	checker := query.NewChecker(bindings, nil)

	role := model.NewRole(wsID, "admin", []string{"governance:admin"}, nil)
	require.NoError(t, roles.Save(ctx, role))
	require.NoError(t, bindings.Save(ctx, model.NewRoleBinding(role.ID, memberID)))

	ok, err := checker.HasPermission(ctx, wsID, memberID, "governance:admin")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = checker.HasPermission(ctx, wsID, memberID, "project:write")
	require.NoError(t, err)
	require.False(t, ok)

	err = checker.RequirePermission(ctx, wsID, memberID, "project:write")
	require.ErrorIs(t, err, domain.ErrPermissionDenied)
}

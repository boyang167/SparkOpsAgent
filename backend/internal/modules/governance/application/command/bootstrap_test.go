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

func (r *memoryRoleRepo) ListWorkspaceIDs(_ context.Context) ([]kernel.WorkspaceID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := make(map[string]kernel.WorkspaceID)
	for _, role := range r.roles {
		seen[role.WorkspaceID.String()] = role.WorkspaceID
	}
	out := make([]kernel.WorkspaceID, 0, len(seen))
	for _, workspaceID := range seen {
		out = append(out, workspaceID)
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

func (r *memoryBindingRepo) Exists(_ context.Context, roleID kernel.RoleID, memberID kernel.MemberID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, binding := range r.bindings {
		if binding.RoleID == roleID && binding.MemberID == memberID {
			return true, nil
		}
	}
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

func TestBootstrapWorkspaceCreatesDefaultRolesAndOnlyBindsAdmin(t *testing.T) {
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
	require.Len(t, items, 5)
	byName := make(map[string]*model.Role, len(items))
	for _, item := range items {
		byName[item.Name] = item
	}
	require.Contains(t, byName, "admin")
	require.Contains(t, byName, "developer")
	require.Contains(t, byName, "reviewer")
	require.Contains(t, byName, "operator")
	require.Contains(t, byName, "viewer")
	require.Contains(t, byName["admin"].Permissions, authz.PermGovernanceAdmin)
	require.Contains(t, byName["developer"].Permissions, authz.PermProjectSubmit)
	require.Contains(t, byName["reviewer"].Permissions, authz.PermProjectReview)
	require.Len(t, bindings.bindings, 1)
	require.Equal(t, byName["admin"].ID, bindings.bindings[0].RoleID)
	require.Equal(t, ownerID, bindings.bindings[0].MemberID)

	err = handler.Handle(ctx, wsID, ownerID, authz.DefaultAdminPermissions)
	require.NoError(t, err)
	items, err = roles.ListByWorkspace(ctx, wsID)
	require.NoError(t, err)
	require.Len(t, items, 5)
	require.Len(t, bindings.bindings, 1)
}

func TestEnsureDefaultWorkspaceRolesAddsMissingRolesWithoutOverwritingExisting(t *testing.T) {
	ctx := context.Background()
	wsID := kernel.NewWorkspaceID()
	roles := newMemoryRoleRepo()
	admin := model.NewRole(wsID, "admin", []string{authz.PermGovernanceAdmin}, nil)
	developer := model.NewRole(wsID, "developer", []string{authz.PermProjectRead}, nil)
	require.NoError(t, roles.Save(ctx, admin))
	require.NoError(t, roles.Save(ctx, developer))

	handler := command.NewEnsureDefaultWorkspaceRolesHandler(roles)
	require.NoError(t, handler.Handle(ctx))

	items, err := roles.ListByWorkspace(ctx, wsID)
	require.NoError(t, err)
	require.Len(t, items, 5)
	byName := make(map[string]*model.Role, len(items))
	for _, item := range items {
		byName[item.Name] = item
	}
	require.Equal(t, []string{authz.PermProjectRead}, byName["developer"].Permissions)
	require.Contains(t, byName["admin"].Permissions, authz.PermProjectReview)
	require.Contains(t, byName["reviewer"].Permissions, authz.PermProjectReview)
}

func TestEnsureDefaultWorkspaceRolesSeedsWorkspaceWithoutAdmin(t *testing.T) {
	ctx := context.Background()
	wsID := kernel.NewWorkspaceID()
	roles := newMemoryRoleRepo()
	require.NoError(t, roles.Save(ctx, model.NewRole(wsID, "custom", []string{authz.PermProjectRead}, nil)))

	handler := command.NewEnsureDefaultWorkspaceRolesHandler(roles)
	require.NoError(t, handler.Handle(ctx))

	items, err := roles.ListByWorkspace(ctx, wsID)
	require.NoError(t, err)
	require.Len(t, items, 6)
}

func TestUpdateRoleCannotRenameProtectedDefaultRole(t *testing.T) {
	ctx := context.Background()
	wsID := kernel.NewWorkspaceID()
	roles := newMemoryRoleRepo()
	reviewer := model.NewRole(wsID, "reviewer", []string{authz.PermProjectReview}, nil)
	require.NoError(t, roles.Save(ctx, reviewer))
	handler := command.NewUpdateRoleHandler(roles)
	newName := "renamed-reviewer"

	_, err := handler.Handle(ctx, wsID, reviewer.ID, command.UpdateRoleInput{Name: &newName})
	require.ErrorIs(t, err, domain.ErrRoleProtected)
}

package service

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/policy"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

// APIPolicySync keeps Casbin g/p rules aligned with role API policies and member bindings.
type APIPolicySync struct {
	engine   policy.Engine
	roles    repository.RoleRepository
	bindings repository.BindingRepository
	store    *persistence.RoleAPIPolicyStore
}

func NewAPIPolicySync(
	engine policy.Engine,
	roles repository.RoleRepository,
	bindings repository.BindingRepository,
	store *persistence.RoleAPIPolicyStore,
) *APIPolicySync {
	return &APIPolicySync{engine: engine, roles: roles, bindings: bindings, store: store}
}

func (s *APIPolicySync) HasAPIPoliciesForWorkspace(ctx context.Context, workspaceID kernel.WorkspaceID) (bool, error) {
	return s.store.HasAnyForWorkspace(ctx, workspaceID.String())
}

func (s *APIPolicySync) SyncRolePolicies(ctx context.Context, roleID kernel.RoleID, workspaceID kernel.WorkspaceID) error {
	policies, err := s.store.ListByRole(ctx, roleID.String())
	if err != nil {
		return err
	}
	pairs := make([][2]string, 0, len(policies))
	for _, p := range policies {
		pairs = append(pairs, [2]string{p.Path, p.Method})
	}
	return s.engine.SyncRoleAPIPolicies(roleID.String(), workspaceID.String(), pairs)
}

func (s *APIPolicySync) SyncMemberRole(ctx context.Context, workspaceID kernel.WorkspaceID, roleID kernel.RoleID, memberID kernel.MemberID, add bool) error {
	if add {
		return s.engine.AddGroupingPolicy(memberID.String(), roleID.String(), workspaceID.String())
	}
	return s.engine.RemoveGroupingPolicy(memberID.String(), roleID.String(), workspaceID.String())
}

func (s *APIPolicySync) RemoveRolePolicies(ctx context.Context, roleID kernel.RoleID, workspaceID kernel.WorkspaceID) error {
	return s.engine.SyncRoleAPIPolicies(roleID.String(), workspaceID.String(), nil)
}

func (s *APIPolicySync) Bootstrap(ctx context.Context) error {
	roles, err := s.store.ListRolesWithPolicies(ctx)
	if err != nil {
		return err
	}
	for _, row := range roles {
		roleID, err := kernel.ParseRoleID(row.RoleID)
		if err != nil {
			continue
		}
		role, err := s.roles.FindByID(ctx, roleID)
		if err != nil {
			continue
		}
		if err := s.SyncRolePolicies(ctx, roleID, role.WorkspaceID); err != nil {
			return err
		}
	}
	bindings, err := s.store.ListAllBindings(ctx)
	if err != nil {
		return err
	}
	for _, b := range bindings {
		roleID, err1 := kernel.ParseRoleID(b.RoleID)
		memberID, err2 := kernel.ParseMemberID(b.MemberID)
		if err1 != nil || err2 != nil {
			continue
		}
		role, err := s.roles.FindByID(ctx, roleID)
		if err != nil {
			continue
		}
		if err := s.engine.AddGroupingPolicy(memberID.String(), roleID.String(), role.WorkspaceID.String()); err != nil {
			return err
		}
	}
	return s.engine.SavePolicy()
}

var _ interface {
	HasAPIPoliciesForWorkspace(ctx context.Context, workspaceID kernel.WorkspaceID) (bool, error)
} = (*APIPolicySync)(nil)

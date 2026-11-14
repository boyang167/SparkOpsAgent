package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Checker struct {
	bindings    repository.BindingRepository
	systemRoles authz.SystemRoleChecker
}

func NewChecker(bindings repository.BindingRepository, systemRoles authz.SystemRoleChecker) *Checker {
	return &Checker{bindings: bindings, systemRoles: systemRoles}
}

func (c *Checker) HasPermission(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID, perm string) (bool, error) {
	if c.systemRoles != nil {
		isSuper, err := c.systemRoles.IsSuperAdmin(ctx, memberID)
		if err != nil {
			return false, err
		}
		if isSuper {
			return true, nil
		}
	}
	roles, err := c.bindings.ListRolesForMember(ctx, workspaceID, memberID)
	if err != nil {
		return false, err
	}
	for _, role := range roles {
		if role.HasPermission(perm) {
			return true, nil
		}
	}
	return false, nil
}

func (c *Checker) RequirePermission(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID, perm string) error {
	ok, err := c.HasPermission(ctx, workspaceID, memberID, perm)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrPermissionDenied
	}
	return nil
}

func (c *Checker) ListRolesForMember(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID) ([]*model.Role, error) {
	return c.bindings.ListRolesForMember(ctx, workspaceID, memberID)
}

func (c *Checker) ListWorkspaceRoleIDs(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID) ([]kernel.RoleID, error) {
	roles, err := c.bindings.ListRolesForMember(ctx, workspaceID, memberID)
	if err != nil {
		return nil, err
	}
	ids := make([]kernel.RoleID, 0, len(roles))
	for _, role := range roles {
		if role != nil {
			ids = append(ids, role.ID)
		}
	}
	return ids, nil
}

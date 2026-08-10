package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Checker struct {
	bindings repository.BindingRepository
}

func NewChecker(bindings repository.BindingRepository) *Checker {
	return &Checker{bindings: bindings}
}

func (c *Checker) HasPermission(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID, perm string) (bool, error) {
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

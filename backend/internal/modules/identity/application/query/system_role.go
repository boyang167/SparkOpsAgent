package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SystemRoleChecker struct {
	store *persistence.SystemRoleStore
}

func NewSystemRoleChecker(store *persistence.SystemRoleStore) *SystemRoleChecker {
	return &SystemRoleChecker{store: store}
}

func (c *SystemRoleChecker) IsSuperAdmin(ctx context.Context, memberID kernel.MemberID) (bool, error) {
	return c.store.IsSuperAdmin(ctx, memberID)
}

func (c *SystemRoleChecker) GrantSuperAdmin(ctx context.Context, memberID kernel.MemberID) error {
	return c.store.Grant(ctx, memberID, "super_admin")
}

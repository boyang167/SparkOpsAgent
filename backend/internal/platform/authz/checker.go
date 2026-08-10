package authz

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PermissionChecker interface {
	RequirePermission(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID, perm string) error
	HasPermission(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID, perm string) (bool, error)
}

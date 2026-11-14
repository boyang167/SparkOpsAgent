package authz

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PermissionChecker interface {
	RequirePermission(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID, perm string) error
	HasPermission(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID, perm string) (bool, error)
}

type WorkspaceAccessResolver interface {
	ResolveWorkspaceAccess(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID) (kernel.OrgID, bool, error)
}

type WorkspaceScopeResolver interface {
	ResolveWorkspaceOrganization(ctx context.Context, workspaceID kernel.WorkspaceID) (kernel.OrgID, bool, error)
}

type WorkspaceRoleReader interface {
	ListWorkspaceRoleIDs(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID) ([]kernel.RoleID, error)
}

type RoleScopeResolver interface {
	ResolveRoleWorkspace(ctx context.Context, roleID kernel.RoleID) (kernel.WorkspaceID, error)
}

type WorkspaceRolePermissionChecker interface {
	HasWorkspaceRolePermission(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID, permission string) (bool, error)
}

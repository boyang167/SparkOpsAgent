package application

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func modulesAllowed(spec domain.ToolSpec, ents entitlements.WorkspaceEntitlements) bool {
	if spec.PluginID != "" {
		if !ents.AllowsModule("plugin:" + spec.PluginID) {
			return false
		}
	}
	if len(spec.Modules) == 0 {
		return true
	}
	for _, moduleKey := range spec.Modules {
		if !ents.AllowsModule(moduleKey) {
			return false
		}
	}
	return true
}

func hasToolPermission(
	ctx context.Context,
	checker authz.PermissionChecker,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	perm string,
) (bool, error) {
	if perm == "" {
		return true, nil
	}
	return checker.HasPermission(ctx, wsID, memberID, perm)
}

func toolVisible(
	ctx context.Context,
	checker authz.PermissionChecker,
	ents entitlements.WorkspaceEntitlements,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	spec domain.ToolSpec,
) (bool, error) {
	if !modulesAllowed(spec, ents) {
		return false, nil
	}
	return hasToolPermission(ctx, checker, wsID, memberID, spec.Permission)
}

func requireToolAccess(
	ctx context.Context,
	checker authz.PermissionChecker,
	ents entitlements.WorkspaceEntitlements,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	spec domain.ToolSpec,
) error {
	if !modulesAllowed(spec, ents) {
		return errToolModuleDisabled
	}
	if spec.Permission == "" {
		return nil
	}
	return checker.RequirePermission(ctx, wsID, memberID, spec.Permission)
}

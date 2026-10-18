package bootstrap

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type governanceWriteAuthorizer struct {
	checker authz.PermissionChecker
}

func newGovernanceWriteAuthorizer(checker authz.PermissionChecker) sparkplugin.WriteAuthorizer {
	return governanceWriteAuthorizer{checker: checker}
}

func (a governanceWriteAuthorizer) CanWrite(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) (bool, error) {
	if a.checker == nil {
		return false, nil
	}
	return a.checker.HasPermission(ctx, wsID, memberID, authz.PermGovernanceAdmin)
}

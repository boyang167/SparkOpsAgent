package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkspaceChecker interface {
	MemberInWorkspace(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID) (bool, error)
	CanCreateProject(ctx context.Context, wsID kernel.WorkspaceID, currentCount int) error
	OrgIDForWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (kernel.OrgID, error)
}

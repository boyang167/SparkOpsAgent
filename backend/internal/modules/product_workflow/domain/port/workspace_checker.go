package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkspaceChecker interface {
	MemberInWorkspace(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID) (bool, error)
}

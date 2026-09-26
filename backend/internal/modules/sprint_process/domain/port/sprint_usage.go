package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SprintUsageChecker interface {
	CountSprintsByProcess(ctx context.Context, processID kernel.SprintProcessID) (int, error)
}

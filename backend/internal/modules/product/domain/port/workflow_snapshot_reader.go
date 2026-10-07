package port

import (
	"context"
	"encoding/json"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkflowSnapshotReader interface {
	GetWorkflowSnapshot(ctx context.Context, workflowID kernel.ProductWorkflowID) (json.RawMessage, error)
}

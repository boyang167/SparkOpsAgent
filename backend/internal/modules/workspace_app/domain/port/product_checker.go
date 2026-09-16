package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProductChecker interface {
	ProductInWorkspace(ctx context.Context, wsID kernel.WorkspaceID, productID kernel.ProductID) (bool, error)
}

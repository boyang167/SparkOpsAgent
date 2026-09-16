package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProductChecker interface {
	ProductWorkspace(ctx context.Context, productID kernel.ProductID) (kernel.WorkspaceID, error)
	ProductName(ctx context.Context, productID kernel.ProductID) (string, error)
}

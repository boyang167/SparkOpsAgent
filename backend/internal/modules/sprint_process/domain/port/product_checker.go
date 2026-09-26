package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProductChecker interface {
	ProductExists(ctx context.Context, productID kernel.ProductID) error
}

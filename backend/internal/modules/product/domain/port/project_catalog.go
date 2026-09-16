package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProductProjectView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	Status       string `json:"status"`
	ReviewStatus string `json:"review_status"`
	ProductID    string `json:"product_id,omitempty"`
}

type ProjectCatalogPort interface {
	ListProjectsByProduct(ctx context.Context, wsID kernel.WorkspaceID, productID kernel.ProductID) ([]ProductProjectView, error)
	CountProjectsByProduct(ctx context.Context, wsID kernel.WorkspaceID, productID kernel.ProductID) (int, error)
}

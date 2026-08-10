package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProductRepository interface {
	SaveProduct(ctx context.Context, product *model.Product) error
	UpdateProduct(ctx context.Context, product *model.Product) error
	FindProductByID(ctx context.Context, id kernel.ProductID) (*model.Product, error)
	ListProductsByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Product, error)
	SavePlan(ctx context.Context, plan *model.ProductPlan) error
	ListPlansByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.ProductPlan, error)
	SaveRequirement(ctx context.Context, req *model.ProductRequirement) error
	FindRequirementByID(ctx context.Context, id kernel.ProductRequirementID) (*model.ProductRequirement, error)
	FindRequirementByBuildID(ctx context.Context, buildID kernel.BuildID) (*model.ProductRequirement, error)
	UpdateRequirement(ctx context.Context, req *model.ProductRequirement) error
	ListRequirementsByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.ProductRequirement, error)
}

package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProductOverviewView struct {
	ProductID        string `json:"product_id"`
	Name             string `json:"name"`
	DisplayName      string `json:"display_name"`
	Description      string `json:"description"`
	Status           string `json:"status"`
	ProjectCount     int    `json:"project_count"`
	EnvironmentCount int    `json:"environment_count"`
}

type ListProductProjectsHandler struct {
	products repository.ProductRepository
	catalog  port.ProjectCatalogPort
}

func NewListProductProjectsHandler(products repository.ProductRepository, catalog port.ProjectCatalogPort) *ListProductProjectsHandler {
	return &ListProductProjectsHandler{products: products, catalog: catalog}
}

func (h *ListProductProjectsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, productID kernel.ProductID) ([]port.ProductProjectView, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	return h.catalog.ListProjectsByProduct(ctx, wsID, productID)
}

type GetProductOverviewHandler struct {
	products     repository.ProductRepository
	catalog      port.ProjectCatalogPort
	environments repository.EnvironmentRepository
}

func NewGetProductOverviewHandler(
	products repository.ProductRepository,
	catalog port.ProjectCatalogPort,
	environments repository.EnvironmentRepository,
) *GetProductOverviewHandler {
	return &GetProductOverviewHandler{products: products, catalog: catalog, environments: environments}
}

func (h *GetProductOverviewHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, productID kernel.ProductID) (*ProductOverviewView, error) {
	p, err := h.products.FindProductByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	count, err := h.catalog.CountProjectsByProduct(ctx, wsID, productID)
	if err != nil {
		return nil, err
	}
	envCount, err := h.environments.CountEnvironmentsByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	return &ProductOverviewView{
		ProductID: p.ID.String(),
		Name:      p.Name.String(), DisplayName: p.DisplayName, Description: p.Description,
		ProjectCount: count, EnvironmentCount: envCount,
	}, nil
}

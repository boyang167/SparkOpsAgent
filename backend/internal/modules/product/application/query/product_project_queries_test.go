package query_test

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type stubProductRepo struct {
	product *model.Product
}

func (s stubProductRepo) SaveProduct(context.Context, *model.Product) error { return nil }
func (s stubProductRepo) UpdateProduct(context.Context, *model.Product) error { return nil }
func (s stubProductRepo) FindProductByID(_ context.Context, id kernel.ProductID) (*model.Product, error) {
	if s.product == nil || s.product.ID != id {
		return nil, domain.ErrProductNotFound
	}
	return s.product, nil
}
func (s stubProductRepo) ListProductsByWorkspace(context.Context, kernel.WorkspaceID) ([]*model.Product, error) {
	return nil, nil
}
func (s stubProductRepo) SavePlan(context.Context, *model.ProductPlan) error { return nil }
func (s stubProductRepo) ListPlansByProduct(context.Context, kernel.ProductID) ([]*model.ProductPlan, error) {
	return nil, nil
}
func (s stubProductRepo) SaveRequirement(context.Context, *model.ProductRequirement) error { return nil }
func (s stubProductRepo) FindRequirementByID(context.Context, kernel.ProductRequirementID) (*model.ProductRequirement, error) {
	return nil, domain.ErrProductRequirementNotFound
}
func (s stubProductRepo) FindRequirementByBuildID(context.Context, kernel.BuildID) (*model.ProductRequirement, error) {
	return nil, domain.ErrProductRequirementNotFound
}
func (s stubProductRepo) UpdateRequirement(context.Context, *model.ProductRequirement) error { return nil }
func (s stubProductRepo) ListRequirementsByProduct(context.Context, kernel.ProductID) ([]*model.ProductRequirement, error) {
	return nil, nil
}

type stubCatalog struct {
	count int
	items []port.ProductProjectView
}

func (s stubCatalog) ListProjectsByProduct(context.Context, kernel.WorkspaceID, kernel.ProductID) ([]port.ProductProjectView, error) {
	return s.items, nil
}
func (s stubCatalog) CountProjectsByProduct(context.Context, kernel.WorkspaceID, kernel.ProductID) (int, error) {
	return s.count, nil
}

type stubEnvironmentRepo struct{ count int }

func (s stubEnvironmentRepo) SaveEnvironment(context.Context, *model.Environment) error { return nil }
func (s stubEnvironmentRepo) FindEnvironmentByID(context.Context, kernel.EnvironmentID) (*model.Environment, error) {
	return nil, domain.ErrEnvironmentNotFound
}
func (s stubEnvironmentRepo) ListEnvironmentsByProduct(context.Context, kernel.ProductID) ([]*model.Environment, error) {
	return nil, nil
}
func (s stubEnvironmentRepo) DeleteEnvironment(context.Context, kernel.EnvironmentID) error { return nil }
func (s stubEnvironmentRepo) CountEnvironmentsByProduct(context.Context, kernel.ProductID) (int, error) {
	return s.count, nil
}
func (s stubEnvironmentRepo) SaveEnvironmentProject(context.Context, *model.EnvironmentProject) error { return nil }
func (s stubEnvironmentRepo) ListEnvironmentProjects(context.Context, kernel.EnvironmentID) ([]*model.EnvironmentProject, error) {
	return nil, nil
}
func (s stubEnvironmentRepo) EnsureEnvironmentProjects(context.Context, kernel.EnvironmentID, []kernel.ProjectID) error {
	return nil
}
func (s stubEnvironmentRepo) FindEnvironmentProject(context.Context, kernel.EnvironmentID, kernel.ProjectID) (*model.EnvironmentProject, error) {
	return nil, domain.ErrEnvironmentProjectNotFound
}

func TestGetProductOverviewReturnsProjectCount(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	name, err := model.NewProductName("demo-product")
	require.NoError(t, err)
	p := model.NewProduct(wsID, name, "Demo", "desc", "1.0", "1.1", kernel.NewMemberID())

	handler := query.NewGetProductOverviewHandler(
		stubProductRepo{product: p},
		stubCatalog{count: 2},
		stubEnvironmentRepo{count: 1},
	)

	view, err := handler.Handle(context.Background(), wsID, p.ID)
	require.NoError(t, err)
	require.Equal(t, 2, view.ProjectCount)
	require.Equal(t, 1, view.EnvironmentCount)
	require.Equal(t, "demo-product", view.Name)
	require.Equal(t, "Demo", view.DisplayName)
}

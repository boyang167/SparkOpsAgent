package acl

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectCatalogAdapter struct {
	projects repository.ProjectRepository
}

func NewProjectCatalogAdapter(projects repository.ProjectRepository) *ProjectCatalogAdapter {
	return &ProjectCatalogAdapter{projects: projects}
}

func (a *ProjectCatalogAdapter) ListProjectsByProduct(ctx context.Context, _ kernel.WorkspaceID, productID kernel.ProductID) ([]port.ProductProjectView, error) {
	items, err := a.projects.ListByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]port.ProductProjectView, 0, len(items))
	for _, p := range items {
		v := query.ToView(p)
		out = append(out, port.ProductProjectView{
			ID:           v.ID,
			Name:         v.Name,
			DisplayName:  v.DisplayName,
			Status:       v.Status,
			ReviewStatus: v.ReviewStatus,
			ProductID:    v.ProductID,
		})
	}
	return out, nil
}

func (a *ProjectCatalogAdapter) CountProjectsByProduct(ctx context.Context, _ kernel.WorkspaceID, productID kernel.ProductID) (int, error) {
	return a.projects.CountByProduct(ctx, productID)
}

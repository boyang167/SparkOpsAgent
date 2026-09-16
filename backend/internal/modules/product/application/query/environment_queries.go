package query

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListEnvironmentsHandler struct {
	products     repository.ProductRepository
	environments repository.EnvironmentRepository
}

func NewListEnvironmentsHandler(products repository.ProductRepository, environments repository.EnvironmentRepository) *ListEnvironmentsHandler {
	return &ListEnvironmentsHandler{products: products, environments: environments}
}

func (h *ListEnvironmentsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, productID kernel.ProductID) ([]map[string]any, error) {
	product, err := h.products.FindProductByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if product.WorkspaceID != wsID {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	items, err := h.environments.ListEnvironmentsByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, env := range items {
		out = append(out, map[string]any{
			"id": env.ID.String(), "product_id": env.ProductID.String(), "name": env.Name,
			"env_type": env.EnvType, "cluster_id": env.ClusterID.String(), "namespace": env.Namespace,
			"variables": env.Variables, "status": env.Status,
			"created_at": env.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

type GetEnvironmentStatusHandler struct {
	products     repository.ProductRepository
	environments repository.EnvironmentRepository
	catalog      port.ProjectCatalogPort
}

func NewGetEnvironmentStatusHandler(
	products repository.ProductRepository,
	environments repository.EnvironmentRepository,
	catalog port.ProjectCatalogPort,
) *GetEnvironmentStatusHandler {
	return &GetEnvironmentStatusHandler{products: products, environments: environments, catalog: catalog}
}

func (h *GetEnvironmentStatusHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	productID kernel.ProductID,
	envID kernel.EnvironmentID,
) (map[string]any, error) {
	product, err := h.products.FindProductByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if product.WorkspaceID != wsID {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	env, err := h.environments.FindEnvironmentByID(ctx, envID)
	if err != nil {
		return nil, err
	}
	if env.ProductID != productID {
		return nil, domain.ErrEnvironmentNotFound
	}
	projects, err := h.catalog.ListProjectsByProduct(ctx, wsID, productID)
	if err != nil {
		return nil, err
	}
	envProjects, err := h.environments.ListEnvironmentProjects(ctx, envID)
	if err != nil {
		return nil, err
	}
	type projectState struct {
		Enabled                bool
		CurrentImageTag        string
		CurrentDeploymentRunID string
		LastDeployedAt         string
	}
	byProject := make(map[string]projectState, len(envProjects))
	for _, row := range envProjects {
		state := projectState{Enabled: row.Enabled, CurrentImageTag: row.CurrentImageTag}
		if row.CurrentDeploymentRunID != nil {
			state.CurrentDeploymentRunID = row.CurrentDeploymentRunID.String()
		}
		if row.LastDeployedAt != nil {
			state.LastDeployedAt = row.LastDeployedAt.Format(time.RFC3339)
		}
		byProject[row.ProjectID.String()] = state
	}
	matrix := make([]map[string]any, 0, len(projects))
	for _, p := range projects {
		row := map[string]any{
			"project_id": p.ID, "project_name": p.Name, "display_name": p.DisplayName,
			"enabled": true,
		}
		if state, ok := byProject[p.ID]; ok {
			row["enabled"] = state.Enabled
			if state.CurrentImageTag != "" {
				row["current_image_tag"] = state.CurrentImageTag
			}
			if state.CurrentDeploymentRunID != "" {
				row["current_deployment_run_id"] = state.CurrentDeploymentRunID
			}
			if state.LastDeployedAt != "" {
				row["last_deployed_at"] = state.LastDeployedAt
			}
		}
		matrix = append(matrix, row)
	}
	return map[string]any{
		"environment_id": env.ID.String(), "name": env.Name, "namespace": env.Namespace,
		"cluster_id": env.ClusterID.String(), "projects": matrix,
	}, nil
}

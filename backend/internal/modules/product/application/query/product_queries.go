package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListProductsHandler struct {
	repo      repository.ProductRepository
	wsChecker port.WorkspaceChecker
}

func NewListProductsHandler(repo repository.ProductRepository, wsChecker port.WorkspaceChecker) *ListProductsHandler {
	return &ListProductsHandler{repo: repo, wsChecker: wsChecker}
}

func (h *ListProductsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) ([]map[string]any, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	items, err := h.repo.ListProductsByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, p := range items {
		out = append(out, productToMap(p))
	}
	return out, nil
}

type GetProductHandler struct {
	repo      repository.ProductRepository
	wsChecker port.WorkspaceChecker
}

func NewGetProductHandler(repo repository.ProductRepository, wsChecker port.WorkspaceChecker) *GetProductHandler {
	return &GetProductHandler{repo: repo, wsChecker: wsChecker}
}

func (h *GetProductHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, productID kernel.ProductID) (map[string]any, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	p, err := h.repo.FindProductByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if p.WorkspaceID != wsID {
		return nil, domain.ErrProductNotFound
	}
	return productToMap(p), nil
}

type ListProductPlansHandler struct {
	repo repository.ProductRepository
}

func NewListProductPlansHandler(repo repository.ProductRepository) *ListProductPlansHandler {
	return &ListProductPlansHandler{repo: repo}
}

func (h *ListProductPlansHandler) Handle(ctx context.Context, productID kernel.ProductID) ([]map[string]any, error) {
	if _, err := h.repo.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	items, err := h.repo.ListPlansByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, plan := range items {
		out = append(out, map[string]any{
			"id": plan.ID.String(), "product_id": plan.ProductID.String(), "name": plan.Name,
			"version": plan.Version, "status": plan.Status, "description": plan.Description,
			"submitted": plan.Submitted, "other_info": plan.OtherInfo, "created_at": plan.CreatedAt,
		})
	}
	return out, nil
}

type ListProductRequirementsHandler struct {
	repo repository.ProductRepository
}

func NewListProductRequirementsHandler(repo repository.ProductRepository) *ListProductRequirementsHandler {
	return &ListProductRequirementsHandler{repo: repo}
}

func (h *ListProductRequirementsHandler) Handle(ctx context.Context, productID kernel.ProductID) ([]map[string]any, error) {
	if _, err := h.repo.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	items, err := h.repo.ListRequirementsByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, req := range items {
		out = append(out, map[string]any{
			"id": req.ID.String(), "product_id": req.ProductID.String(),
			"product_plan_id": req.ProductPlanID.String(), "project_id": req.ProjectID.String(),
			"project_name": req.ProjectName, "project_tag": req.ProjectTag,
			"build_id": req.BuildID.String(), "description": req.Description,
			"status": req.Status, "multicluster_status": req.MulticlusterStatus,
			"operator": req.Operator, "other_info": req.OtherInfo, "created_at": req.CreatedAt,
		})
	}
	return out, nil
}

func productToMap(p *model.Product) map[string]any {
	return map[string]any{
		"id": p.ID.String(), "workspace_id": p.WorkspaceID.String(), "name": p.Name.String(),
		"display_name": p.DisplayName, "description": p.Description,
		"version": p.Version, "next_version": p.NextVersion, "created_at": p.CreatedAt,
	}
}

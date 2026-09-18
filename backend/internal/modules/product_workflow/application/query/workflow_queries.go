package query

import (
	"context"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListWorkspaceWorkflowsHandler struct {
	workflows repository.ProductWorkflowRepository
	products  port.ProductChecker
	wsChecker port.WorkspaceChecker
}

func NewListWorkspaceWorkflowsHandler(workflows repository.ProductWorkflowRepository, products port.ProductChecker, wsChecker port.WorkspaceChecker) *ListWorkspaceWorkflowsHandler {
	return &ListWorkspaceWorkflowsHandler{workflows: workflows, products: products, wsChecker: wsChecker}
}

func (h *ListWorkspaceWorkflowsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) ([]map[string]any, error) {
	ok, err := h.wsChecker.MemberInWorkspace(ctx, memberID, wsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	items, err := h.workflows.ListWorkflowsByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	names := map[kernel.ProductID]string{}
	out := make([]map[string]any, 0, len(items))
	for _, wf := range items {
		productName := names[wf.ProductID]
		if productName == "" {
			if n, err := h.products.ProductName(ctx, wf.ProductID); err == nil {
				productName = n
				names[wf.ProductID] = n
			}
		}
		out = append(out, map[string]any{
			"id": wf.ID.String(), "product_id": wf.ProductID.String(), "product_name": productName,
			"workspace_id": wf.WorkspaceID.String(), "name": wf.Name, "key": wf.Key,
			"status": wf.Status, "stage_count": len(wf.Stages),
			"created_at": wf.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

type ListProductWorkflowsHandler struct {
	workflows repository.ProductWorkflowRepository
	products  port.ProductChecker
}

func NewListProductWorkflowsHandler(workflows repository.ProductWorkflowRepository, products port.ProductChecker) *ListProductWorkflowsHandler {
	return &ListProductWorkflowsHandler{workflows: workflows, products: products}
}

func (h *ListProductWorkflowsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, productID kernel.ProductID) ([]map[string]any, error) {
	productWs, err := h.products.ProductWorkspace(ctx, productID)
	if err != nil {
		return nil, err
	}
	if productWs != wsID {
		return nil, domain.ErrProductNotInWorkspace
	}
	items, err := h.workflows.ListWorkflowsByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, wf := range items {
		out = append(out, map[string]any{
			"id": wf.ID.String(), "product_id": wf.ProductID.String(), "name": wf.Name,
			"key": wf.Key, "status": wf.Status, "stage_count": len(wf.Stages),
			"created_at": wf.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

type GetProductWorkflowHandler struct {
	workflows repository.ProductWorkflowRepository
	products  port.ProductChecker
}

func NewGetProductWorkflowHandler(workflows repository.ProductWorkflowRepository, products port.ProductChecker) *GetProductWorkflowHandler {
	return &GetProductWorkflowHandler{workflows: workflows, products: products}
}

func (h *GetProductWorkflowHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, productID kernel.ProductID, workflowID kernel.ProductWorkflowID) (map[string]any, error) {
	productWs, err := h.products.ProductWorkspace(ctx, productID)
	if err != nil {
		return nil, err
	}
	if productWs != wsID {
		return nil, domain.ErrProductNotInWorkspace
	}
	wf, err := h.workflows.FindWorkflowByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if wf.ProductID != productID {
		return nil, domain.ErrProductWorkflowNotFound
	}
	return map[string]any{
		"id": wf.ID.String(), "product_id": wf.ProductID.String(), "workspace_id": wf.WorkspaceID.String(),
		"name": wf.Name, "key": wf.Key, "description": wf.Description, "status": wf.Status,
		"stages": wf.Stages, "stage_count": len(wf.Stages),
		"created_at": wf.CreatedAt.Format(time.RFC3339), "updated_at": wf.UpdatedAt.Format(time.RFC3339),
	}, nil
}

type GetWorkflowRunHandler struct {
	workflows repository.ProductWorkflowRepository
	products  port.ProductChecker
}

func NewGetWorkflowRunHandler(workflows repository.ProductWorkflowRepository, products port.ProductChecker) *GetWorkflowRunHandler {
	return &GetWorkflowRunHandler{workflows: workflows, products: products}
}

func (h *GetWorkflowRunHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, productID kernel.ProductID, runID kernel.ProductWorkflowRunID) (map[string]any, error) {
	productWs, err := h.products.ProductWorkspace(ctx, productID)
	if err != nil {
		return nil, err
	}
	if productWs != wsID {
		return nil, domain.ErrProductNotInWorkspace
	}
	run, err := h.workflows.FindRunByID(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.ProductID != productID {
		return nil, domain.ErrProductWorkflowRunNotFound
	}
	out := map[string]any{
		"id": run.ID.String(), "workflow_id": run.WorkflowID.String(), "product_id": run.ProductID.String(),
		"status": run.Status, "current_stage": run.CurrentStage, "stage_runs": run.StageRuns,
		"error_message": run.ErrorMessage, "created_at": run.CreatedAt.Format(time.RFC3339),
	}
	if run.StartedAt != nil {
		out["started_at"] = run.StartedAt.Format(time.RFC3339)
	}
	if run.CompletedAt != nil {
		out["completed_at"] = run.CompletedAt.Format(time.RFC3339)
	}
	return out, nil
}

type ListWorkflowRunsHandler struct {
	workflows repository.ProductWorkflowRepository
	products  port.ProductChecker
}

func NewListWorkflowRunsHandler(workflows repository.ProductWorkflowRepository, products port.ProductChecker) *ListWorkflowRunsHandler {
	return &ListWorkflowRunsHandler{workflows: workflows, products: products}
}

func (h *ListWorkflowRunsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, productID kernel.ProductID, workflowID kernel.ProductWorkflowID) ([]map[string]any, error) {
	productWs, err := h.products.ProductWorkspace(ctx, productID)
	if err != nil {
		return nil, err
	}
	if productWs != wsID {
		return nil, domain.ErrProductNotInWorkspace
	}
	wf, err := h.workflows.FindWorkflowByID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	if wf.ProductID != productID {
		return nil, domain.ErrProductWorkflowNotFound
	}
	runs, err := h.workflows.ListRunsByWorkflow(ctx, workflowID, 20)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(runs))
	for _, run := range runs {
		out = append(out, map[string]any{
			"id": run.ID.String(), "status": run.Status, "current_stage": run.CurrentStage,
			"created_at": run.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

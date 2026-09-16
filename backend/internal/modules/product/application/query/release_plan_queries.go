package query

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListReleasePlansHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewListReleasePlansHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *ListReleasePlansHandler {
	return &ListReleasePlansHandler{products: products, plans: plans}
}

func (h *ListReleasePlansHandler) Handle(ctx context.Context, productID kernel.ProductID) ([]map[string]any, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	items, err := h.plans.ListPlansByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, releasePlanSummary(item))
	}
	return out, nil
}

type GetReleasePlanHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewGetReleasePlanHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *GetReleasePlanHandler {
	return &GetReleasePlanHandler{products: products, plans: plans}
}

func (h *GetReleasePlanHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID) (map[string]any, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	plan, err := h.plans.FindPlanByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan.ProductID != productID {
		return nil, domain.ErrReleasePlanNotFound
	}
	logs, _ := h.plans.ListAuditLogs(ctx, planID, 20)
	out := releasePlanDetail(plan)
	auditItems := make([]map[string]any, 0, len(logs))
	for _, log := range logs {
		var detail any
		_ = json.Unmarshal(log.Detail, &detail)
		auditItems = append(auditItems, map[string]any{
			"id": log.ID, "action": log.Action, "actor_id": log.ActorID.String(),
			"detail": detail, "created_at": log.CreatedAt.Format(time.RFC3339),
		})
	}
	out["audit_logs"] = auditItems
	return out, nil
}

func releasePlanSummary(plan *model.ReleasePlan) map[string]any {
	return map[string]any{
		"id": plan.ID.String(), "product_id": plan.ProductID.String(), "name": plan.Name,
		"description": plan.Description, "status": plan.Status,
		"release_window_start": plan.ReleaseWindowStart.Format(time.RFC3339),
		"release_window_end":   plan.ReleaseWindowEnd.Format(time.RFC3339),
		"approval_enabled":     plan.ApprovalConfig.Enabled,
		"created_at":           plan.CreatedAt.Format(time.RFC3339),
	}
}

func releasePlanDetail(plan *model.ReleasePlan) map[string]any {
	out := releasePlanSummary(plan)
	out["manager_id"] = plan.ManagerID.String()
	jobs := make([]map[string]any, 0, len(plan.Jobs))
	for _, job := range plan.Jobs {
		item := map[string]any{
			"id": job.ID.String(), "name": job.Name, "type": job.Type, "sort_order": job.SortOrder,
			"owner_id": job.OwnerID.String(), "status": job.Status,
			"text_content": job.TextContent, "text_checked": job.TextChecked,
		}
		if len(job.WorkflowRef) > 0 {
			var wfRef any
			_ = json.Unmarshal(job.WorkflowRef, &wfRef)
			item["workflow_ref"] = wfRef
		}
		if len(job.ExecutionRef) > 0 {
			var execRef any
			_ = json.Unmarshal(job.ExecutionRef, &execRef)
			item["execution_ref"] = execRef
		}
		jobs = append(jobs, item)
	}
	out["jobs"] = jobs
	return out
}

package query

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListReleasePlansHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewListReleasePlansHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *ListReleasePlansHandler {
	return &ListReleasePlansHandler{products: products, plans: plans}
}

func (h *ListReleasePlansHandler) Handle(ctx context.Context, productID kernel.ProductID, sprintID *kernel.SprintID) ([]map[string]any, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	var items []*model.ReleasePlan
	var err error
	if sprintID != nil {
		items, err = h.plans.ListPlansByProductAndSprint(ctx, productID, *sprintID)
	} else {
		items, err = h.plans.ListPlansByProduct(ctx, productID)
	}
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
	inputs   appctx.WorkflowRuntimeInputReader
}

func NewGetReleasePlanHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *GetReleasePlanHandler {
	return &GetReleasePlanHandler{products: products, plans: plans}
}

func (h *GetReleasePlanHandler) UseRuntimeInputs(reader appctx.WorkflowRuntimeInputReader) {
	h.inputs = reader
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
	out := releasePlanDetail(plan)
	h.attachRuntimeInputs(ctx, plan, out)
	return out, nil
}

func releasePlanSummary(plan *model.ReleasePlan) map[string]any {
	out := map[string]any{
		"id": plan.ID.String(), "product_id": plan.ProductID.String(), "name": plan.Name,
		"description": plan.Description, "status": plan.Status,
		"release_window_start": plan.ReleaseWindowStart.Format(time.RFC3339),
		"release_window_end":   plan.ReleaseWindowEnd.Format(time.RFC3339),
		"approval_enabled":     plan.ApprovalConfig.Enabled,
		"approval_config":      plan.ApprovalConfig,
		"hook_settings":        plan.HookSettings,
		"hook_state":           plan.HookState,
		"created_at":           plan.CreatedAt.Format(time.RFC3339),
		"version":              plan.Version,
	}
	if plan.ScheduleExecuteTime != nil {
		out["schedule_execute_time"] = plan.ScheduleExecuteTime.Format(time.RFC3339)
	}
	if plan.SprintID != nil {
		out["sprint_id"] = plan.SprintID.String()
	}
	return out
}

// releasePlanListItem 列表视图：不含需求说明、审批详情与 Hook 配置/状态，详情接口按需加载。
func releasePlanListItem(plan *model.ReleasePlan) map[string]any {
	out := map[string]any{
		"id": plan.ID.String(), "product_id": plan.ProductID.String(), "name": plan.Name,
		"status":               plan.Status,
		"release_window_start": plan.ReleaseWindowStart.Format(time.RFC3339),
		"release_window_end":   plan.ReleaseWindowEnd.Format(time.RFC3339),
		"approval_enabled":     plan.ApprovalConfig.Enabled,
		"manager_id":           plan.ManagerID.String(),
		"created_at":           plan.CreatedAt.Format(time.RFC3339),
		"version":              plan.Version,
	}
	if plan.ScheduleExecuteTime != nil {
		out["schedule_execute_time"] = plan.ScheduleExecuteTime.Format(time.RFC3339)
	}
	if plan.SprintID != nil {
		out["sprint_id"] = plan.SprintID.String()
	}
	return out
}

func releasePlanDetail(plan *model.ReleasePlan) map[string]any {
	out := releasePlanSummary(plan)
	out["version"] = plan.Version
	out["manager_id"] = plan.ManagerID.String()
	jobs := make([]map[string]any, 0, len(plan.Jobs))
	for _, job := range plan.Jobs {
		item := map[string]any{
			"id": job.ID.String(), "name": job.Name, "type": job.Type, "sort_order": job.SortOrder,
			"owner_id": job.OwnerID.String(), "status": job.Status,
			"text_content": job.TextContent, "text_format": job.TextFormat, "text_checked": job.TextChecked,
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
	out["parameters"] = plan.Parameters
	if plan.Parameters == nil {
		out["parameters"] = map[string]string{}
	}
	return out
}

func (h *GetReleasePlanHandler) attachRuntimeInputs(ctx context.Context, plan *model.ReleasePlan, out map[string]any) {
	if h.inputs == nil {
		return
	}
	jobs, _ := out["jobs"].([]map[string]any)
	secret := map[string]struct{}{}
	for i := range plan.Jobs {
		job := &plan.Jobs[i]
		if job.Type != model.ReleasePlanJobTypeWorkflow || i >= len(jobs) {
			continue
		}
		_, inputs, err := service.ReleasePlanJobInputs(ctx, plan, job, h.inputs)
		if err != nil {
			continue
		}
		jobs[i]["runtime_inputs"] = inputs
		jobSecret := map[string]struct{}{}
		for _, input := range inputs {
			if input.Secret {
				secret[input.Key] = struct{}{}
				jobSecret[input.Key] = struct{}{}
			}
		}
		maskJobParameters(jobs[i], jobSecret)
	}
	out["parameters"] = model.MaskParameters(plan.Parameters, secret)
}

func maskJobParameters(item map[string]any, secret map[string]struct{}) {
	ref, ok := item["workflow_ref"].(map[string]any)
	if !ok {
		return
	}
	params, ok := ref["parameters"].(map[string]any)
	if !ok {
		return
	}
	for key := range secret {
		if value, ok := params[key].(string); ok && value != "" {
			params[key] = model.MaskedParameterValue
		}
	}
}

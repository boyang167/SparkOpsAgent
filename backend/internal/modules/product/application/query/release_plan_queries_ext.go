package query

import (
	"context"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ReleasePlanListQuery struct {
	SprintID *kernel.SprintID
	Statuses []model.ReleasePlanStatus
	From     *time.Time
	To       *time.Time
}

func ParseReleasePlanListQuery(statusRaw string, fromRaw, toRaw string) (ReleasePlanListQuery, error) {
	q := ReleasePlanListQuery{}
	if statusRaw != "" {
		for _, part := range strings.Split(statusRaw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			q.Statuses = append(q.Statuses, model.ReleasePlanStatus(part))
		}
	}
	if fromRaw != "" {
		t, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			return q, err
		}
		q.From = &t
	}
	if toRaw != "" {
		t, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			return q, err
		}
		q.To = &t
	}
	return q, nil
}

func (h *ListReleasePlansHandler) HandleFiltered(ctx context.Context, productID kernel.ProductID, query ReleasePlanListQuery) ([]map[string]any, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	filter := repository.ReleasePlanListFilter{Statuses: query.Statuses, From: query.From, To: query.To}
	items, err := h.plans.ListPlansByProductFiltered(ctx, productID, query.SprintID, filter)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, releasePlanSummary(item))
	}
	return out, nil
}

type ListWorkspaceReleasePlansHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
	sprints  repository.SprintRepository
}

func NewListWorkspaceReleasePlansHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository, sprints repository.SprintRepository) *ListWorkspaceReleasePlansHandler {
	return &ListWorkspaceReleasePlansHandler{products: products, plans: plans, sprints: sprints}
}

func (h *ListWorkspaceReleasePlansHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, query ReleasePlanListQuery) ([]map[string]any, error) {
	filter := repository.ReleasePlanListFilter{Statuses: query.Statuses, From: query.From, To: query.To}
	items, err := h.plans.ListPlansByWorkspace(ctx, wsID, filter)
	if err != nil {
		return nil, err
	}
	productNames := map[kernel.ProductID]string{}
	sprintNames := map[kernel.SprintID]string{}
	out := make([]map[string]any, 0, len(items))
	for _, plan := range items {
		row := releasePlanSummary(plan)
		name := productNames[plan.ProductID]
		if name == "" {
			if p, err := h.products.FindProductByID(ctx, plan.ProductID); err == nil {
				name = p.DisplayName
				if name == "" {
					name = p.Name.String()
				}
				productNames[plan.ProductID] = name
			}
		}
		row["product_name"] = name
		if plan.SprintID != nil {
			sprintName := sprintNames[*plan.SprintID]
			if sprintName == "" {
				if sprint, err := h.sprints.FindSprintByID(ctx, *plan.SprintID); err == nil {
					sprintName = sprint.Name
					sprintNames[*plan.SprintID] = sprintName
				}
			}
			row["sprint_name"] = sprintName
		}
		out = append(out, row)
	}
	return out, nil
}

type GetReleasePlanHandlerWithSprint struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
	sprints  repository.SprintRepository
	base     *GetReleasePlanHandler
}

func NewGetReleasePlanHandlerWithSprint(products repository.ProductRepository, plans repository.ReleasePlanRepository, sprints repository.SprintRepository) *GetReleasePlanHandlerWithSprint {
	return &GetReleasePlanHandlerWithSprint{
		products: products, plans: plans, sprints: sprints,
		base: NewGetReleasePlanHandler(products, plans),
	}
}

func (h *GetReleasePlanHandlerWithSprint) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID) (map[string]any, error) {
	out, err := h.base.Handle(ctx, productID, planID)
	if err != nil {
		return nil, err
	}
	plan, err := h.plans.FindPlanByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan.SprintID == nil {
		return out, nil
	}
	sprint, err := h.sprints.FindSprintByID(ctx, *plan.SprintID)
	if err != nil {
		return out, nil
	}
	sprintVM := map[string]any{
		"id": sprint.ID.String(), "name": sprint.Name, "status": sprint.Status,
	}
	if len(sprint.ExternalRef) > 0 {
		sprintVM["external_ref"] = sprint.ExternalRef
	}
	out["sprint"] = sprintVM
	items, err := h.sprints.ListWorkItemsBySprint(ctx, *plan.SprintID)
	if err != nil {
		return out, nil
	}
	linked := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row := map[string]any{
			"id": item.ID.String(), "title": item.Title, "stage_id": item.StageID, "status": item.Status,
		}
		if len(item.ExternalRef) > 0 {
			row["pm_links"] = []map[string]any{pmLinkFromExternalRef(item.ExternalRef)}
		}
		linked = append(linked, row)
	}
	out["linked_work_items"] = linked
	return out, nil
}

func pmLinkFromExternalRef(ref map[string]any) map[string]any {
	link := map[string]any{}
	if provider, ok := ref["provider"].(string); ok {
		link["provider"] = provider
	}
	if key, ok := ref["external_key"].(string); ok {
		link["external_key"] = key
	}
	if url, ok := ref["url"].(string); ok {
		link["url"] = url
	}
	return link
}

type ListBlockingReleasePlansHandler struct {
	plans repository.ReleasePlanRepository
}

func NewListBlockingReleasePlansHandler(plans repository.ReleasePlanRepository) *ListBlockingReleasePlansHandler {
	return &ListBlockingReleasePlansHandler{plans: plans}
}

func (h *ListBlockingReleasePlansHandler) Handle(ctx context.Context, sprintID kernel.SprintID) ([]map[string]any, error) {
	items, err := h.plans.ListBlockingPlansBySprint(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, plan := range items {
		out = append(out, map[string]any{
			"id": plan.ID.String(), "name": plan.Name, "status": plan.Status,
			"product_id": plan.ProductID.String(),
		})
	}
	return out, nil
}

func IsReleasePlanNotFound(err error) bool {
	return err == domain.ErrReleasePlanNotFound
}

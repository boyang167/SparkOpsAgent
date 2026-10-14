package query

import (
	"context"
	"sort"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListSprintsHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewListSprintsHandler(products repository.ProductRepository, sprints repository.SprintRepository) *ListSprintsHandler {
	return &ListSprintsHandler{products: products, sprints: sprints}
}

func (h *ListSprintsHandler) Handle(ctx context.Context, productID kernel.ProductID, status string) ([]map[string]any, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	items, err := h.sprints.ListSprintsByProduct(ctx, productID, status)
	if err != nil {
		return nil, err
	}
	counts, err := h.sprints.CountWorkItemsByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		count := counts[item.ID.String()]
		out = append(out, sprintSummary(item, count[0], count[1]))
	}
	return out, nil
}

type GetSprintHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewGetSprintHandler(products repository.ProductRepository, sprints repository.SprintRepository) *GetSprintHandler {
	return &GetSprintHandler{products: products, sprints: sprints}
}

func (h *GetSprintHandler) Handle(ctx context.Context, productID kernel.ProductID, sprintID kernel.SprintID) (map[string]any, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	sprint, err := h.sprints.FindSprintByID(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint.ProductID != productID {
		return nil, domain.ErrSprintNotFound
	}
	total, done, _ := h.sprints.CountWorkItemsBySprint(ctx, sprint.ID)
	return sprintDetail(sprint, total, done), nil
}

type GetSprintBoardHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewGetSprintBoardHandler(products repository.ProductRepository, sprints repository.SprintRepository) *GetSprintBoardHandler {
	return &GetSprintBoardHandler{products: products, sprints: sprints}
}

func (h *GetSprintBoardHandler) Handle(ctx context.Context, productID kernel.ProductID, sprintID kernel.SprintID) (map[string]any, error) {
	if _, err := h.products.FindProductByID(ctx, productID); err != nil {
		return nil, err
	}
	sprint, err := h.sprints.FindSprintByID(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if sprint.ProductID != productID {
		return nil, domain.ErrSprintNotFound
	}
	items, err := h.sprints.ListWorkItemCardsBySprint(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	total, done := 0, 0
	byStage := map[string][]*model.WorkItem{}
	for _, item := range items {
		total++
		if item.Status == model.WorkItemStatusDone {
			done++
		}
		byStage[item.StageID] = append(byStage[item.StageID], item)
	}
	stages := append([]model.SprintStage(nil), sprint.Stages...)
	sort.Slice(stages, func(i, j int) bool { return stages[i].SortOrder < stages[j].SortOrder })
	columns := make([]map[string]any, 0, len(stages))
	for _, stage := range stages {
		stageItems := byStage[stage.ID]
		cards := make([]map[string]any, 0, len(stageItems))
		for _, item := range stageItems {
			cards = append(cards, workItemCard(item))
		}
		wipCount := len(stageItems)
		col := map[string]any{
			"stage":      stage,
			"work_items": cards,
			"wip_count":  wipCount,
		}
		if stage.WipLimit > 0 && wipCount > stage.WipLimit {
			col["wip_limit_exceeded"] = true
		}
		columns = append(columns, col)
	}
	return map[string]any{
		"sprint":  sprintDetail(sprint, total, done),
		"columns": columns,
	}, nil
}

type GetWorkItemHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewGetWorkItemHandler(products repository.ProductRepository, sprints repository.SprintRepository) *GetWorkItemHandler {
	return &GetWorkItemHandler{products: products, sprints: sprints}
}

func (h *GetWorkItemHandler) Handle(ctx context.Context, productID kernel.ProductID, workItemID kernel.WorkItemID) (map[string]any, error) {
	item, err := h.sprints.FindWorkItemByID(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	sprint, err := h.sprints.FindSprintByID(ctx, item.SprintID)
	if err != nil || sprint.ProductID != productID {
		return nil, domain.ErrWorkItemNotFound
	}
	return workItemDetail(item), nil
}

func sprintSummary(s *model.Sprint, total, done int) map[string]any {
	out := map[string]any{
		"id": s.ID.String(), "product_id": s.ProductID.String(), "name": s.Name,
		"goal": s.Goal, "status": s.Status, "process_version": s.ProcessVersion,
		"process_id": s.ProcessID.String(), "work_item_count": total, "done_work_item_count": done,
		"created_at": s.CreatedAt.Format(time.RFC3339), "updated_at": s.UpdatedAt.Format(time.RFC3339),
	}
	if s.OwnerID != nil {
		out["owner_id"] = s.OwnerID.String()
	}
	if s.PlannedStart != nil {
		out["planned_start"] = s.PlannedStart.Format(time.RFC3339)
	}
	if s.PlannedEnd != nil {
		out["planned_end"] = s.PlannedEnd.Format(time.RFC3339)
	}
	if s.ActualStart != nil {
		out["actual_start"] = s.ActualStart.Format(time.RFC3339)
	}
	if s.ActualEnd != nil {
		out["actual_end"] = s.ActualEnd.Format(time.RFC3339)
	}
	if len(s.ExternalRef) > 0 {
		out["external"] = s.ExternalRef
	}
	return out
}

func sprintDetail(s *model.Sprint, total, done int) map[string]any {
	out := sprintSummary(s, total, done)
	out["stages"] = s.Stages
	return out
}

func workItemCard(item *model.WorkItem) map[string]any {
	out := map[string]any{
		"id": item.ID.String(), "sprint_id": item.SprintID.String(), "stage_id": item.StageID,
		"sort_order": item.SortOrder, "type": item.Type, "title": item.Title,
		"priority": item.Priority, "status": item.Status, "version": item.Version,
		"project_ids": item.ProjectIDs, "labels": item.Labels,
		"updated_at": item.UpdatedAt.Format(time.RFC3339),
	}
	if item.ParentID != nil {
		out["parent_id"] = item.ParentID.String()
	}
	if item.AssigneeID != nil {
		out["assignee_id"] = item.AssigneeID.String()
	}
	if len(item.ExternalRef) > 0 {
		out["external"] = item.ExternalRef
	}
	if len(item.Delivery) > 0 {
		out["delivery"] = item.Delivery
	}
	if item.BlockedReason != "" {
		out["blocked_reason"] = item.BlockedReason
	}
	return out
}

type ListWorkItemExecutionsHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewListWorkItemExecutionsHandler(products repository.ProductRepository, sprints repository.SprintRepository) *ListWorkItemExecutionsHandler {
	return &ListWorkItemExecutionsHandler{products: products, sprints: sprints}
}

func (h *ListWorkItemExecutionsHandler) Handle(ctx context.Context, productID kernel.ProductID, workItemID kernel.WorkItemID) ([]map[string]any, error) {
	item, err := h.sprints.FindWorkItemByID(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	sprint, err := h.sprints.FindSprintByID(ctx, item.SprintID)
	if err != nil {
		return nil, err
	}
	if sprint.ProductID != productID {
		return nil, domain.ErrWorkItemNotFound
	}
	execs, err := h.sprints.ListExecutionsByWorkItem(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(execs))
	for _, exec := range execs {
		out = append(out, workItemExecutionVM(exec))
	}
	return out, nil
}

func workItemExecutionVM(exec *model.WorkItemExecution) map[string]any {
	out := map[string]any{
		"id": exec.ID.String(), "work_item_id": exec.WorkItemID.String(),
		"stage_id": exec.StageID, "workflow_id": exec.WorkflowID.String(),
		"workflow_run_id": exec.WorkflowRunID.String(), "triggered_by": exec.TriggeredBy.String(),
		"status": exec.Status, "started_at": exec.StartedAt.Format(time.RFC3339),
	}
	if exec.ErrorMessage != "" {
		out["error_message"] = exec.ErrorMessage
	}
	if exec.FinishedAt != nil {
		out["finished_at"] = exec.FinishedAt.Format(time.RFC3339)
	}
	if len(exec.Artifacts) > 0 {
		out["artifacts"] = exec.Artifacts
	}
	return out
}

func workItemDetail(item *model.WorkItem) map[string]any {
	out := workItemCard(item)
	out["description"] = item.Description
	out["created_by"] = item.CreatedBy.String()
	out["created_at"] = item.CreatedAt.Format(time.RFC3339)
	if item.ReporterID != nil {
		out["reporter_id"] = item.ReporterID.String()
	}
	return out
}

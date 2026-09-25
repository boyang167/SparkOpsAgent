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

type GetSprintMetricsHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewGetSprintMetricsHandler(products repository.ProductRepository, sprints repository.SprintRepository) *GetSprintMetricsHandler {
	return &GetSprintMetricsHandler{products: products, sprints: sprints}
}

func (h *GetSprintMetricsHandler) Handle(ctx context.Context, productID kernel.ProductID, sprintID kernel.SprintID) (map[string]any, error) {
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
	items, err := h.sprints.ListWorkItemsBySprint(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	var done, blocked int
	for _, item := range items {
		switch item.Status {
		case model.WorkItemStatusDone:
			done++
		case model.WorkItemStatusBlocked:
			blocked++
		}
	}
	out := map[string]any{
		"sprint_id":          sprintID.String(),
		"total_work_items":   len(items),
		"done_work_items":    done,
		"blocked_work_items": blocked,
	}
	if burndown := buildBurndown(sprint, items); len(burndown) > 0 {
		out["burndown"] = burndown
	}
	if cycle := buildStageCycleHours(ctx, h.sprints, items); len(cycle) > 0 {
		out["stage_cycle_time_hours"] = cycle
	}
	var runs, failures int
	for _, item := range items {
		execs, err := h.sprints.ListExecutionsByWorkItem(ctx, item.ID)
		if err != nil {
			continue
		}
		for _, exec := range execs {
			runs++
			if exec.Status == model.WorkItemExecutionFailed {
				failures++
			}
		}
	}
	if runs > 0 {
		out["deployment_frequency"] = float64(runs)
		out["change_failure_rate"] = float64(failures) / float64(runs)
	}
	return out, nil
}

func buildBurndown(sprint *model.Sprint, items []*model.WorkItem) []map[string]any {
	start := sprint.CreatedAt
	if sprint.ActualStart != nil {
		start = *sprint.ActualStart
	}
	end := time.Now().UTC()
	if sprint.ActualEnd != nil {
		end = *sprint.ActualEnd
	}
	if end.Before(start) {
		return nil
	}
	type dayKey struct {
		y, m, d int
	}
	createdByDay := map[dayKey]int{}
	doneByDay := map[dayKey]int{}
	for _, item := range items {
		createdByDay[dayKey{item.CreatedAt.Year(), int(item.CreatedAt.Month()), item.CreatedAt.Day()}]++
		if item.Status == model.WorkItemStatusDone {
			doneByDay[dayKey{item.UpdatedAt.Year(), int(item.UpdatedAt.Month()), item.UpdatedAt.Day()}]++
		}
	}
	var points []map[string]any
	total := 0
	for d := start.Truncate(24 * time.Hour); !d.After(end); d = d.Add(24 * time.Hour) {
		key := dayKey{d.Year(), int(d.Month()), d.Day()}
		total += createdByDay[key]
		total -= doneByDay[key]
		if total < 0 {
			total = 0
		}
		points = append(points, map[string]any{
			"date":      d.Format("2006-01-02"),
			"remaining": total,
		})
	}
	return points
}

func buildStageCycleHours(ctx context.Context, sprints repository.SprintRepository, items []*model.WorkItem) map[string]float64 {
	type acc struct {
		totalHours float64
		count      int
	}
	byStage := map[string]*acc{}
	for _, item := range items {
		execs, err := sprints.ListExecutionsByWorkItem(ctx, item.ID)
		if err != nil {
			continue
		}
		for _, exec := range execs {
			if exec.Status != model.WorkItemExecutionSuccess || exec.FinishedAt == nil {
				continue
			}
			hours := exec.FinishedAt.Sub(exec.StartedAt).Hours()
			if hours < 0 {
				continue
			}
			if byStage[exec.StageID] == nil {
				byStage[exec.StageID] = &acc{}
			}
			byStage[exec.StageID].totalHours += hours
			byStage[exec.StageID].count++
		}
	}
	out := map[string]float64{}
	for stageID, a := range byStage {
		if a.count > 0 {
			out[stageID] = a.totalHours / float64(a.count)
		}
	}
	return out
}

type ListActiveSprintsHandler struct {
	products repository.ProductRepository
	sprints  repository.SprintRepository
}

func NewListActiveSprintsHandler(products repository.ProductRepository, sprints repository.SprintRepository) *ListActiveSprintsHandler {
	return &ListActiveSprintsHandler{products: products, sprints: sprints}
}

func (h *ListActiveSprintsHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID) ([]map[string]any, error) {
	sprints, err := h.sprints.ListActiveSprintsByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	products, err := h.products.ListProductsByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	nameByProduct := map[kernel.ProductID]string{}
	for _, p := range products {
		name := p.DisplayName
		if name == "" {
			name = p.Name.String()
		}
		nameByProduct[p.ID] = name
	}
	sort.Slice(sprints, func(i, j int) bool {
		return sprints[i].UpdatedAt.After(sprints[j].UpdatedAt)
	})
	out := make([]map[string]any, 0, len(sprints))
	for _, sprint := range sprints {
		total, done, _ := h.sprints.CountWorkItemsBySprint(ctx, sprint.ID)
		row := sprintSummary(sprint, total, done)
		row["product_name"] = nameByProduct[sprint.ProductID]
		out = append(out, row)
	}
	return out, nil
}

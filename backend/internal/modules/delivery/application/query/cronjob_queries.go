package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListCronJobsHandler struct {
	repo    *persistence.CronJobRepository
	checker port.ProjectChecker
}

func NewListCronJobsHandler(repo *persistence.CronJobRepository, checker port.ProjectChecker) *ListCronJobsHandler {
	return &ListCronJobsHandler{repo: repo, checker: checker}
}

func (h *ListCronJobsHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
) ([]map[string]any, error) {
	inWs, err := h.checker.ProjectInWorkspace(ctx, projectID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrProjectAccessDenied
	}
	items, err := h.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id": item.ID.String(), "project_id": item.ProjectID.String(),
			"name": item.Name, "schedule": item.Schedule, "branch": item.Branch,
			"enabled": item.Enabled, "created_at": item.CreatedAt,
		})
	}
	return out, nil
}

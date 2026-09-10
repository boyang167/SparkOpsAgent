package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListWebhooksHandler struct {
	repo    *persistence.WebhookRepository
	checker port.ProjectChecker
}

func NewListWebhooksHandler(repo *persistence.WebhookRepository, checker port.ProjectChecker) *ListWebhooksHandler {
	return &ListWebhooksHandler{repo: repo, checker: checker}
}

func (h *ListWebhooksHandler) Handle(
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
		out = append(out, WebhookJSON(item))
	}
	return out, nil
}

func WebhookJSON(item persistence.WebhookRecord) map[string]any {
	events := item.Events
	if events == nil {
		events = []string{}
	}
	return map[string]any{
		"id": item.ID.String(), "project_id": item.ProjectID.String(),
		"name": item.Name, "url": item.URL, "token": item.Token,
		"events": events, "enabled": item.Enabled,
		"created_at": item.CreatedAt.UTC().Format("2006-01-02 15:04:05"),
	}
}

func WebhookEventCatalog() []map[string]string {
	return []map[string]string{
		{"key": command.WebhookEventBuild, "label": "Build-构建"},
		{"key": command.WebhookEventDeploy, "label": "Deploy-发布"},
		{"key": command.WebhookEventRollback, "label": "Rollback-回滚"},
	}
}

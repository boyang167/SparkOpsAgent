package query

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const releasePlanHistoryPageSize = 20

type ListReleasePlanAuditLogsHandler struct {
	products repository.ProductRepository
	plans    repository.ReleasePlanRepository
}

func NewListReleasePlanAuditLogsHandler(products repository.ProductRepository, plans repository.ReleasePlanRepository) *ListReleasePlanAuditLogsHandler {
	return &ListReleasePlanAuditLogsHandler{products: products, plans: plans}
}

func (h *ListReleasePlanAuditLogsHandler) Handle(ctx context.Context, productID kernel.ProductID, planID kernel.ReleasePlanID, offset int) (map[string]any, error) {
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
	logs, err := h.plans.ListAuditLogsPage(ctx, planID, releasePlanHistoryPageSize+1, offset)
	if err != nil {
		return nil, err
	}
	hasMore := len(logs) > releasePlanHistoryPageSize
	if hasMore {
		logs = logs[:releasePlanHistoryPageSize]
	}
	items := make([]map[string]any, 0, len(logs))
	for _, log := range logs {
		var detail any
		_ = json.Unmarshal(log.Detail, &detail)
		items = append(items, map[string]any{
			"id": log.ID, "action": log.Action, "actor_id": log.ActorID.String(),
			"detail": detail, "created_at": log.CreatedAt.Format(time.RFC3339),
		})
	}
	return map[string]any{"items": items, "has_more": hasMore}, nil
}

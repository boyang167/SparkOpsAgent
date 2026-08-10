package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ListAuditLogsHandler struct {
	store audit.Store
}

func NewListAuditLogsHandler(store audit.Store) *ListAuditLogsHandler {
	return &ListAuditLogsHandler{store: store}
}

func (h *ListAuditLogsHandler) HandlePlatform(ctx context.Context, limit, offset int) ([]audit.LogView, error) {
	return h.store.List(ctx, audit.ListFilter{Limit: limit, Offset: offset})
}

func (h *ListAuditLogsHandler) HandleOrg(ctx context.Context, orgID kernel.OrgID, limit, offset int) ([]audit.LogView, error) {
	return h.store.List(ctx, audit.ListFilter{OrgID: &orgID, Limit: limit, Offset: offset})
}

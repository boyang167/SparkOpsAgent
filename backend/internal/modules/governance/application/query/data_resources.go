package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/persistence"
)

type ListDataResourcesHandler struct {
	store *persistence.DataPolicyStore
}

func NewListDataResourcesHandler(store *persistence.DataPolicyStore) *ListDataResourcesHandler {
	return &ListDataResourcesHandler{store: store}
}

func (h *ListDataResourcesHandler) Handle(ctx context.Context) ([]persistence.DataResource, error) {
	return h.store.ListResources(ctx)
}

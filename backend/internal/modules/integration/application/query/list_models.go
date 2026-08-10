package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/integration/domain/port"
)

type ListModelsHandler struct {
	llm port.LLMClient
}

func NewListModelsHandler(llm port.LLMClient) *ListModelsHandler {
	return &ListModelsHandler{llm: llm}
}

func (h *ListModelsHandler) Handle(ctx context.Context) ([]port.ModelInfo, error) {
	return h.llm.ListModels(ctx)
}

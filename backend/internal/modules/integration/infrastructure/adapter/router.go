package adapter

import (
	"context"
	"strings"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/integration/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/integration/infrastructure/adapter/mock"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/integration/infrastructure/adapter/openai"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
)

type Router struct {
	openai *openai.Client
	mock   *mock.Client
}

func NewRouter(cfg config.LLMConfig) port.LLMClient {
	r := &Router{mock: mock.NewClient()}
	if p, ok := cfg.Providers["openai"]; ok && strings.TrimSpace(p.APIKey) != "" {
		r.openai = openai.NewClient(p.APIKey, p.BaseURL)
	}
	return r
}

func (r *Router) ChatStream(ctx context.Context, req port.ChatRequest, onDelta func(string) error) error {
	if r.openai != nil && !strings.HasPrefix(req.Model, "mock-") {
		return r.openai.ChatStream(ctx, req, onDelta)
	}
	return r.mock.ChatStream(ctx, req, onDelta)
}

func (r *Router) ListModels(ctx context.Context) ([]port.ModelInfo, error) {
	if r.openai != nil {
		models, err := r.openai.ListModels(ctx)
		if err != nil {
			return nil, err
		}
		mockModels, _ := r.mock.ListModels(ctx)
		return append(models, mockModels...), nil
	}
	return r.mock.ListModels(ctx)
}

var _ port.LLMClient = (*Router)(nil)

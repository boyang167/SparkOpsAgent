package integration

import (
	"github.com/sparkops/sparkops-agent/backend/internal/modules/integration/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/integration/infrastructure/adapter"
	integrationhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/integration/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "integration" }

func (m *Module) Register(c *appctx.Container) error {
	llm := adapter.NewRouter(c.Config.LLM)
	c.LLMClient = llm

	handler := integrationhttp.NewHandler(query.NewListModelsHandler(llm))
	auth := authn.RequireAuth(m.tokens)
	c.Engine.GET("/api/v1/models", auth, handler.ListModels)
	return nil
}

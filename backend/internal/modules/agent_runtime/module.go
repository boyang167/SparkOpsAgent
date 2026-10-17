package agent_runtime

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/application/query"
	runtimeevent "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/stream"
	runtimehttp "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "agent_runtime" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("agent_runtime module requires database")
	}
	if err := runtimeevent.Register(c.EventRegistry); err != nil {
		return fmt.Errorf("agent_runtime event codecs: %w", err)
	}
	if c.LLMClient == nil {
		return fmt.Errorf("agent_runtime module requires LLM client")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("agent_runtime module requires permission checker")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("agent_runtime migrate: %w", err)
	}

	store := persistence.NewStore(c.DB)
	conversations := persistence.NewConversationRepository(store)
	messages := persistence.NewMessageRepository(store)
	runs := persistence.NewRunRepository(store)
	agents := acl.NewAgentACL(c.DB)
	knowledge := acl.NewKnowledgeACL(c.DB)
	hub := stream.NewHub()

	handler := runtimehttp.NewHandler(
		command.NewStartChatHandler(conversations, messages, runs, agents, knowledge, c.LLMClient, hub, c.EventBus, c.Transactor, c.Logger),
		query.NewGetConversationHandler(conversations, messages),
		query.NewListAgentConversationsHandler(conversations, messages),
		query.NewStreamRunHandler(conversations, runs, hub),
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)

	c.Engine.POST("/api/v1/agents/:agentId/chat",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermChatUse), handler.StartChat)
	c.Engine.GET("/api/v1/runs/:runId/stream",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermChatUse), handler.StreamRun)
	c.Engine.GET("/api/v1/conversations/:id",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermChatUse), handler.GetConversation)
	c.Engine.GET("/api/v1/agents/:agentId/conversations",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermChatUse), handler.ListAgentConversations)

	return nil
}

package mcp

import (
	"fmt"

	agentquery "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/application/query"
	agentpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/infrastructure/persistence"
	agentacl "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/infrastructure/acl"
	runtimecommand "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/application/command"
	runtimeacl "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/acl"
	runtimepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/persistence"
	runtimestream "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/stream"
	knowledgequery "github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/application/query"
	knowledgepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/persistence"
	knowledgeacl "github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/application"
	mcphttp "github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/interface/http"
	workspacequery "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	workspacepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/persistence"
	workspaceacl "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/acl"
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

func (m *Module) Name() string { return "mcp" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("mcp module requires database")
	}
	if c.LLMClient == nil {
		return fmt.Errorf("mcp module requires LLM client")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("mcp module requires permission checker")
	}

	projectRepo := workspacepersistence.NewProjectRepository(c.DB)
	projectMemberRepo := workspacepersistence.NewProjectMemberRepository(c.DB)
	wsChecker := workspaceacl.NewTenantACL(c.DB)
	memberLookup := workspaceacl.NewMemberLookup(c.DB)

	kbRepo := knowledgepersistence.NewKnowledgeRepository(c.DB)
	kbWsChecker := knowledgeacl.NewTenantACL(c.DB)

	agentRepo := agentpersistence.NewAgentRepository(c.DB)
	projectChecker := agentacl.NewWorkspaceAppACL(c.DB)

	runtimeStore := runtimepersistence.NewStore(c.DB)
	conversations := runtimepersistence.NewConversationRepository(runtimeStore)
	messages := runtimepersistence.NewMessageRepository(runtimeStore)
	runs := runtimepersistence.NewRunRepository(runtimeStore)
	agents := runtimeacl.NewAgentACL(c.DB)
	knowledge := runtimeacl.NewKnowledgeACL(c.DB)
	hub := runtimestream.NewHub()

	toolExecutor := application.NewToolExecutor(
		workspacequery.NewListProjectsHandler(projectRepo, projectMemberRepo, wsChecker, c.PermissionChecker, memberLookup),
		knowledgequery.NewListKnowledgeBasesHandler(kbRepo, kbWsChecker),
		knowledgequery.NewSearchDocumentsHandler(kbRepo, kbWsChecker),
		agentquery.NewListAgentsHandler(agentRepo, projectChecker),
		runtimecommand.NewStartChatHandler(
			conversations, messages, runs, agents, knowledge, c.LLMClient, hub, c.EventBus, c.Logger,
		),
		runs,
	)

	handler := mcphttp.NewHandler(application.NewRPCHandler(toolExecutor))

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)
	projectRead := authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead)
	chatUse := authz.RequirePermission(c.PermissionChecker, authz.PermChatUse)

	c.Engine.GET("/api/v1/mcp/tools",
		auth, ws, wsMember, projectRead, handler.ListTools)
	c.Engine.POST("/api/v1/mcp/rpc",
		auth, ws, wsMember, projectRead, chatUse, handler.HandleRPC)

	return nil
}

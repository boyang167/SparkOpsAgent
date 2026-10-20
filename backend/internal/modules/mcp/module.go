package mcp

import (
	"fmt"
	"time"

	agentquery "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/application/query"
	agentacl "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/infrastructure/acl"
	agentpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/infrastructure/persistence"
	runtimecommand "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/application/command"
	runtimequery "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/application/query"
	runtimeacl "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/acl"
	runtimepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/persistence"
	runtimestream "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/stream"
	knowledgequery "github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/application/query"
	knowledgeacl "github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/acl"
	knowledgepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/application"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/domain"
	mcphttp "github.com/sparkops/sparkops-agent/backend/internal/modules/mcp/interface/http"
	adminacl "github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/infrastructure/acl"
	adminpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/infrastructure/persistence"
	productcommand "github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	productquery "github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/query"
	productacl "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/acl"
	productpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/persistence"
	pwfquery "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/query"
	pwfacl "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/acl"
	pwfpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/persistence"
	workspacequery "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
	workspaceacl "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/acl"
	workspacepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/persistence"
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

	registry := domain.NewRegistry()
	application.RegisterBuiltinTools(registry, application.BuiltinDeps{
		ListProjects: workspacequery.NewListProjectsHandler(projectRepo, projectMemberRepo, wsChecker, c.PermissionChecker, memberLookup),
		GetProject: workspacequery.NewGetProjectHandler(projectRepo, projectMemberRepo, wsChecker, c.PermissionChecker, memberLookup),
		ListKBs:      knowledgequery.NewListKnowledgeBasesHandler(kbRepo, kbWsChecker),
		SearchDocs:   knowledgequery.NewSearchDocumentsHandler(kbRepo, kbWsChecker),
		ListAgents:   agentquery.NewListAgentsHandler(agentRepo, projectChecker),
		StartChat: runtimecommand.NewStartChatHandler(
			conversations, messages, runs, agents, knowledge, c.LLMClient, hub, c.EventBus, c.Transactor, c.Logger,
		),
		GetChatRun:    runtimequery.NewGetChatRunHandler(conversations, runs),
		Runs:          runs,
		ChatWaitLimit: 30 * time.Second,
	})

	productRepo := productpersistence.NewProductRepository(c.DB)
	sprintRepo := productpersistence.NewSprintRepository(c.DB)
	productWsChecker := productacl.NewTenantACL(c.DB)
	workflowRepo := pwfpersistence.NewProductWorkflowRepository(c.DB)
	productChecker := pwfacl.NewProductChecker(c.DB)
	application.RegisterActionTools(registry, application.ActionDeps{
		ListProducts:    productquery.NewListProductsHandler(productRepo, productWsChecker),
		ListSprints:     productquery.NewListSprintsHandler(productRepo, sprintRepo),
		ListWorkflows:   pwfquery.NewListProductWorkflowsHandler(workflowRepo, productChecker),
		WorkflowTrigger: c.WorkflowRunTrigger,
		CreateWorkItem:  productcommand.NewCreateWorkItemHandler(productRepo, sprintRepo, nil),
	})

	adminRepo := adminpersistence.NewAdminRepository(c.DB)
	pluginContext := sparkplugin.PluginContext{
		Logger:   c.Logger,
		DB:       c.DB,
		Clusters: sparkplugin.NewFactory(adminacl.NewClusterCredentialSource(adminRepo)),
	}
	application.RegisterPluginTools(registry, pluginContext)

	entsProvider := application.DBEntitlementsProvider{DB: c.DB}
	toolExecutor := application.NewToolExecutor(registry, c.PermissionChecker, entsProvider)
	listTools := application.NewListToolsHandler(registry, c.PermissionChecker, entsProvider)
	handler := mcphttp.NewHandler(
		application.NewRPCHandler(toolExecutor, listTools),
		listTools,
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)

	c.Engine.GET("/api/v1/mcp/tools",
		auth, ws, wsMember, handler.ListTools)
	c.Engine.POST("/api/v1/mcp/rpc",
		auth, ws, wsMember, handler.HandleRPC)
	c.Engine.GET("/api/v1/mcp",
		auth, ws, wsMember, handler.HandleStreamable)
	c.Engine.POST("/api/v1/mcp",
		auth, ws, wsMember, handler.HandleStreamable)
	c.Engine.DELETE("/api/v1/mcp",
		auth, ws, wsMember, handler.HandleStreamable)

	return nil
}

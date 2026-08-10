package agent

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/infrastructure/persistence"
	agenthttp "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/interface/http"
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

func (m *Module) Name() string { return "agent" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("agent module requires database")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("agent module requires permission checker")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("agent migrate: %w", err)
	}

	repo := persistence.NewAgentRepository(c.DB)
	projectChecker := acl.NewWorkspaceAppACL(c.DB)
	quotaChecker := acl.NewTenantACL(c.DB)

	handler := agenthttp.NewHandler(
		command.NewCreateAgentHandler(repo, projectChecker, quotaChecker, c.EventBus),
		command.NewUpdateAgentHandler(repo, projectChecker),
		command.NewArchiveAgentHandler(repo, projectChecker),
		command.NewAddVersionHandler(repo, projectChecker),
		command.NewPublishAgentHandler(repo, projectChecker, c.EventBus),
		query.NewListAgentsHandler(repo, projectChecker),
		query.NewGetAgentHandler(repo, projectChecker),
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)

	c.Engine.POST("/api/v1/projects/:projectId/agents",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermAgentWrite), handler.CreateAgent)
	c.Engine.GET("/api/v1/projects/:projectId/agents",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermAgentRead), handler.ListAgents)
	c.Engine.GET("/api/v1/agents/:agentId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermAgentRead), handler.GetAgent)
	c.Engine.PATCH("/api/v1/agents/:agentId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermAgentWrite), handler.UpdateAgent)
	c.Engine.DELETE("/api/v1/agents/:agentId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermAgentWrite), handler.ArchiveAgent)
	c.Engine.POST("/api/v1/agents/:agentId/versions",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermAgentWrite), handler.AddVersion)
	c.Engine.POST("/api/v1/agents/:agentId/publish",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermAgentPublish), handler.Publish)

	return nil
}

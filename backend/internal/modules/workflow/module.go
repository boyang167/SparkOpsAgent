package workflow

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/application/query"
	workflowevent "github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/infrastructure/persistence"
	workflowhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/interface/http"
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

func (m *Module) Name() string { return "workflow" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("workflow module requires database")
	}
	if err := workflowevent.Register(c.EventRegistry); err != nil {
		return fmt.Errorf("workflow event codecs: %w", err)
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("workflow module requires permission checker")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("workflow migrate: %w", err)
	}

	repo := persistence.NewWorkflowRepository(c.DB)
	runRepo := persistence.NewWorkflowRunRepository(c.DB)
	projectChecker := acl.NewWorkspaceAppACL(c.DB)

	handler := workflowhttp.NewHandler(
		command.NewCreateWorkflowHandler(repo, projectChecker),
		command.NewUpdateWorkflowHandler(repo, projectChecker),
		command.NewActivateWorkflowHandler(repo, projectChecker),
		command.NewArchiveWorkflowHandler(repo, projectChecker),
		command.NewStartWorkflowRunHandler(repo, runRepo, projectChecker, c.EventBus, c.Transactor),
		query.NewListWorkflowsHandler(repo, projectChecker),
		query.NewGetWorkflowHandler(repo, projectChecker),
		query.NewListWorkflowRunsHandler(repo, runRepo, projectChecker),
		query.NewGetWorkflowRunHandler(repo, runRepo, projectChecker),
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)

	c.Engine.POST("/api/v1/projects/:projectId/workflows",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateWorkflow)
	c.Engine.GET("/api/v1/projects/:projectId/workflows",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListWorkflows)
	c.Engine.GET("/api/v1/workflows/:workflowId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetWorkflow)
	c.Engine.PATCH("/api/v1/workflows/:workflowId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateWorkflow)
	c.Engine.DELETE("/api/v1/workflows/:workflowId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ArchiveWorkflow)
	c.Engine.POST("/api/v1/workflows/:workflowId/activate",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ActivateWorkflow)
	c.Engine.POST("/api/v1/workflows/:workflowId/runs",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.StartWorkflowRun)
	c.Engine.GET("/api/v1/workflows/:workflowId/runs",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListWorkflowRuns)
	c.Engine.GET("/api/v1/workflow-runs/:runId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetWorkflowRun)

	return nil
}

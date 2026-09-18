package product_workflow

import (
	"context"
	"fmt"
	"log"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/subscriber"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/trigger"
	pwfhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "product_workflow" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("product_workflow module requires database")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("product_workflow module requires permission checker")
	}
	if c.BuildTrigger == nil {
		return fmt.Errorf("product_workflow module requires build trigger")
	}
	if c.DeploymentTrigger == nil {
		return fmt.Errorf("product_workflow module requires deployment trigger")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("product_workflow migrate: %w", err)
	}
	if err := command.SyncBuiltinTemplates(context.Background(), c.DB); err != nil {
		log.Printf("product_workflow: sync builtin templates: %v", err)
	}

	templateRepo := persistence.NewWorkflowTemplateRepository(c.DB)
	workflowRepo := persistence.NewProductWorkflowRepository(c.DB)
	wsChecker := acl.NewTenantACL(c.DB)
	productChecker := acl.NewProductChecker(c.DB)
	buildExecutor := engine.NewBuildExecutor(c.BuildTrigger, workflowRepo, acl.NewRegistryResolver(c.DB))
	deployExecutor := engine.NewDeployExecutor(c.DeploymentTrigger, workflowRepo)
	scheduler := engine.NewScheduler(workflowRepo, buildExecutor, deployExecutor, c.EventBus)

	startRun := command.NewStartWorkflowRunHandler(workflowRepo, scheduler, productChecker)
	startRun.UseGitLookup(acl.NewProjectGitLookup(c.DB))
	trigger.NewScheduler(workflowRepo, startRun, c.Logger).Start(context.Background())
	workflowHandlers := pwfhttp.WorkflowHandlers{
		CreateFromTemplate: command.NewCreateWorkflowFromTemplateHandler(templateRepo, workflowRepo, productChecker),
		CreateBlank:        command.NewCreateBlankWorkflowHandler(workflowRepo, productChecker),
		Update:             command.NewUpdateWorkflowHandler(workflowRepo, productChecker),
		Delete:             command.NewDeleteWorkflowHandler(workflowRepo, productChecker),
		GetWorkflow:        query.NewGetProductWorkflowHandler(workflowRepo, productChecker),
		StartRun:           startRun,
		ListWorkflows:      query.NewListProductWorkflowsHandler(workflowRepo, productChecker),
		ListRuns:           query.NewListWorkflowRunsHandler(workflowRepo, productChecker),
		GetRun:             query.NewGetWorkflowRunHandler(workflowRepo, productChecker),
		ApproveJob:         command.NewApproveWorkflowJobHandler(workflowRepo, scheduler, productChecker),
		RejectJob:          command.NewRejectWorkflowJobHandler(workflowRepo, scheduler, productChecker),
		ContinueStage:      command.NewContinueManualStageHandler(workflowRepo, scheduler, productChecker),
	}

	handler := pwfhttp.NewHandler(
		query.NewListWorkflowTemplatesHandler(templateRepo, wsChecker),
		query.NewGetWorkflowTemplateHandler(templateRepo, wsChecker),
		query.NewPreviewWorkflowTemplateHandler(templateRepo),
		query.NewListWorkspaceWorkflowsHandler(workflowRepo, productChecker, wsChecker),
		command.NewCreateWorkflowTemplateHandler(templateRepo, wsChecker),
		command.NewUpdateWorkflowTemplateHandler(templateRepo, wsChecker),
		command.NewPublishWorkflowTemplateHandler(templateRepo, wsChecker),
		workflowHandlers,
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)
	readPerm := authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead)
	writePerm := authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite)

	c.Engine.GET("/api/v1/workspaces/:wsId/workflow-templates",
		auth, ws, wsMember, readPerm, handler.ListTemplates)
	c.Engine.POST("/api/v1/workspaces/:wsId/workflow-templates",
		auth, ws, wsMember, writePerm, handler.CreateTemplate)
	c.Engine.GET("/api/v1/workspaces/:wsId/workflow-templates/:templateId",
		auth, ws, wsMember, readPerm, handler.GetTemplate)
	c.Engine.PUT("/api/v1/workspaces/:wsId/workflow-templates/:templateId",
		auth, ws, wsMember, writePerm, handler.UpdateTemplate)
	c.Engine.POST("/api/v1/workspaces/:wsId/workflow-templates/:templateId/publish",
		auth, ws, wsMember, writePerm, handler.PublishTemplate)
	c.Engine.POST("/api/v1/workspaces/:wsId/workflow-templates/:templateId/preview",
		auth, ws, wsMember, readPerm, handler.PreviewTemplate)
	c.Engine.GET("/api/v1/workspaces/:wsId/workflows",
		auth, ws, wsMember, readPerm, handler.ListWorkspaceWorkflows)

	c.Engine.GET("/api/v1/products/:productId/workflows",
		auth, ws, wsMember, readPerm, handler.ListProductWorkflows)
	c.Engine.POST("/api/v1/products/:productId/workflows",
		auth, ws, wsMember, writePerm, handler.CreateBlankWorkflow)
	c.Engine.POST("/api/v1/products/:productId/workflows/from-template",
		auth, ws, wsMember, writePerm, handler.CreateWorkflowFromTemplate)
	c.Engine.GET("/api/v1/products/:productId/workflows/:workflowId",
		auth, ws, wsMember, readPerm, handler.GetProductWorkflow)
	c.Engine.PATCH("/api/v1/products/:productId/workflows/:workflowId",
		auth, ws, wsMember, writePerm, handler.UpdateProductWorkflow)
	c.Engine.DELETE("/api/v1/products/:productId/workflows/:workflowId",
		auth, ws, wsMember, writePerm, handler.DeleteProductWorkflow)
	c.Engine.POST("/api/v1/products/:productId/workflows/:workflowId/runs",
		auth, ws, wsMember, writePerm, handler.StartWorkflowRun)
	c.Engine.GET("/api/v1/products/:productId/workflows/:workflowId/runs",
		auth, ws, wsMember, readPerm, handler.ListWorkflowRuns)
	c.Engine.GET("/api/v1/products/:productId/workflows/:workflowId/runs/:runId",
		auth, ws, wsMember, readPerm, handler.GetWorkflowRun)
	c.Engine.POST("/api/v1/products/:productId/workflows/:workflowId/runs/:runId/continue",
		auth, ws, wsMember, writePerm, handler.ContinueWorkflowStage)

	c.Engine.POST("/api/v1/products/:productId/workflows/:workflowId/hooks/:token", handler.WorkflowWebhook)
	c.Engine.POST("/api/v1/public/workflows/gitlab", handler.GitLabWorkflowHook)

	c.EventBus.Subscribe(deliveryevent.BuildCompletedName, subscriber.NewOnBuildCompleted(scheduler).Handle)
	c.EventBus.Subscribe(deliveryevent.DeployCompletedName, subscriber.NewOnDeployCompleted(scheduler).Handle)

	c.WorkflowRunTrigger = &workflowRunTriggerPort{start: startRun, products: productChecker}

	c.Engine.POST("/api/v1/products/:productId/workflows/:workflowId/runs/:runId/jobs/:jobName/approve",
		auth, ws, wsMember, writePerm, handler.ApproveWorkflowJob)
	c.Engine.POST("/api/v1/products/:productId/workflows/:workflowId/runs/:runId/jobs/:jobName/reject",
		auth, ws, wsMember, writePerm, handler.RejectWorkflowJob)

	return nil
}

type workflowRunTriggerPort struct {
	start    *command.StartWorkflowRunHandler
	products port.ProductChecker
}

func (p *workflowRunTriggerPort) StartForProduct(ctx context.Context, productID kernel.ProductID, workflowID kernel.ProductWorkflowID) (kernel.ProductWorkflowRunID, error) {
	wsID, err := p.products.ProductWorkspace(ctx, productID)
	if err != nil {
		return kernel.ProductWorkflowRunID{}, err
	}
	run, err := p.start.Handle(ctx, wsID, productID, workflowID, nil)
	if err != nil {
		return kernel.ProductWorkflowRunID{}, err
	}
	return run.ID, nil
}

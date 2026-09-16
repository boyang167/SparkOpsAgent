package product

import (
	"fmt"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	pwfevent "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/subscriber"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/persistence"
	producthttp "github.com/sparkops/sparkops-agent/backend/internal/modules/product/interface/http"
	wspersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/persistence"
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

func (m *Module) Name() string { return "product" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("product module requires database")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("product module requires permission checker")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("product migrate: %w", err)
	}

	repo := persistence.NewProductRepository(c.DB)
	releasePlanRepo := persistence.NewReleasePlanRepository(c.DB)
	environmentRepo := persistence.NewEnvironmentRepository(c.DB)
	projectRepo := wspersistence.NewProjectRepository(c.DB)
	projectCatalog := acl.NewProjectCatalogAdapter(projectRepo)
	wsChecker := acl.NewTenantACL(c.DB)

	environmentHandlers := producthttp.EnvironmentHandlers{
		CreateEnvironment: command.NewCreateEnvironmentHandler(repo, environmentRepo, projectCatalog, c.DB),
		UpdateEnvironment: command.NewUpdateEnvironmentHandler(repo, environmentRepo, c.DB),
		DeleteEnvironment: command.NewDeleteEnvironmentHandler(repo, environmentRepo),
		ListEnvironments:  query.NewListEnvironmentsHandler(repo, environmentRepo),
		GetStatus:         query.NewGetEnvironmentStatusHandler(repo, environmentRepo, projectCatalog),
	}

	releasePlanExecutor := service.NewReleasePlanExecutor(releasePlanRepo, c.WorkflowRunTrigger)
	releasePlanHandlers := producthttp.NewReleasePlanHandlers(
		command.NewCreateReleasePlanHandler(repo, releasePlanRepo),
		command.NewAddReleasePlanJobHandler(repo, releasePlanRepo),
		command.NewFinishReleasePlanPlanningHandler(repo, releasePlanRepo, releasePlanExecutor),
		command.NewApproveReleasePlanHandler(repo, releasePlanRepo, releasePlanExecutor),
		command.NewRejectReleasePlanHandler(repo, releasePlanRepo),
		command.NewMarkReleasePlanJobDoneHandler(repo, releasePlanRepo, releasePlanExecutor),
		command.NewSkipReleasePlanJobHandler(repo, releasePlanRepo, releasePlanExecutor),
		command.NewExecuteReleasePlanHandler(repo, releasePlanRepo, releasePlanExecutor),
		command.NewRetryReleasePlanJobHandler(repo, releasePlanRepo, releasePlanExecutor),
		query.NewListReleasePlansHandler(repo, releasePlanRepo),
		query.NewGetReleasePlanHandler(repo, releasePlanRepo),
	)

	handler := producthttp.NewHandler(
		command.NewCreateProductHandler(repo, wsChecker),
		command.NewUpdateProductHandler(repo, wsChecker),
		command.NewCreateProductPlanHandler(repo),
		command.NewCreateProductRequirementHandler(repo, releasePlanRepo),
		command.NewApproveRequirementHandler(repo),
		command.NewRejectRequirementHandler(repo),
		command.NewReleaseRequirementHandler(repo, c.BuildTrigger),
		query.NewListProductsHandler(repo, wsChecker),
		query.NewGetProductHandler(repo, wsChecker),
		query.NewListProductPlansHandler(repo),
		query.NewListProductRequirementsHandler(repo),
		query.NewListProductProjectsHandler(repo, projectCatalog),
		query.NewGetProductOverviewHandler(repo, projectCatalog, environmentRepo),
		releasePlanHandlers,
		environmentHandlers,
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)

	c.Engine.GET("/api/v1/workspaces/:wsId/products",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListProducts)
	c.Engine.POST("/api/v1/workspaces/:wsId/products",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateProduct)
	c.Engine.GET("/api/v1/products/:productId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetProduct)
	c.Engine.GET("/api/v1/products/:productId/overview",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetProductOverview)
	c.Engine.GET("/api/v1/products/:productId/projects",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListProductProjects)
	c.Engine.PATCH("/api/v1/products/:productId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateProduct)
	c.Engine.GET("/api/v1/products/:productId/plans",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListPlans)
	c.Engine.POST("/api/v1/products/:productId/plans",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreatePlan)
	c.Engine.GET("/api/v1/products/:productId/requirements",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListRequirements)
	c.Engine.POST("/api/v1/products/:productId/requirements",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateRequirement)
	c.Engine.POST("/api/v1/products/:productId/requirements/:reqId/approve",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ApproveRequirement)
	c.Engine.POST("/api/v1/products/:productId/requirements/:reqId/reject",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.RejectRequirement)
	c.Engine.POST("/api/v1/products/:productId/requirements/:reqId/release",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ReleaseRequirement)

	c.Engine.GET("/api/v1/products/:productId/release-plans",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListReleasePlans)
	c.Engine.POST("/api/v1/products/:productId/release-plans",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateReleasePlan)
	c.Engine.GET("/api/v1/products/:productId/release-plans/:planId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetReleasePlan)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/finish-planning",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.FinishReleasePlanPlanning)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/approve",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ApproveReleasePlan)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/reject",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.RejectReleasePlan)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/jobs",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.AddReleasePlanJob)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/jobs/:jobId/done",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.MarkReleasePlanJobDone)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/jobs/:jobId/skip",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.SkipReleasePlanJob)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/execute",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ExecuteReleasePlan)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/jobs/:jobId/retry",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.RetryReleasePlanJob)

	c.EventBus.Subscribe(deliveryevent.BuildCompletedName, subscriber.NewOnBuildCompleted(repo).Handle)
	c.EventBus.Subscribe(pwfevent.WorkflowRunCompletedName, subscriber.NewOnWorkflowRunCompleted(releasePlanExecutor).Handle)
	c.EventBus.Subscribe(deliveryevent.DeployCompletedName, subscriber.NewOnDeployCompleted(environmentRepo).Handle)

	c.Engine.GET("/api/v1/products/:productId/environments",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListEnvironments)
	c.Engine.POST("/api/v1/products/:productId/environments",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateEnvironment)
	c.Engine.PATCH("/api/v1/products/:productId/environments/:envId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateEnvironment)
	c.Engine.DELETE("/api/v1/products/:productId/environments/:envId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.DeleteEnvironment)
	c.Engine.GET("/api/v1/products/:productId/environments/:envId/status",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetEnvironmentStatus)

	return nil
}

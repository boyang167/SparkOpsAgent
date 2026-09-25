package product

import (
	"context"
	"fmt"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	pwfevent "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/subscriber"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/persistence"
	producthttp "github.com/sparkops/sparkops-agent/backend/internal/modules/product/interface/http"
	wspersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
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
	sprintRepo := persistence.NewSprintRepository(c.DB)
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

	secretBox, err := crypto.NewSecretBoxFromConfig(c.Config)
	if err != nil {
		return fmt.Errorf("product jira secret box: %w", err)
	}
	jiraConnector := acl.NewJiraConnector(c.DB, secretBox)
	pmRouter := port.NewPmConnectorRouter(
		acl.NewJiraPmAdapter(jiraConnector),
		acl.NewTapdConnector(c.DB, secretBox),
		acl.NewStubPmConnector(port.PmProviderPingCode),
		acl.NewStubPmConnector(port.PmProviderFeishu),
	)
	boardHub := service.NewSprintBoardHub()
	boardNotifier := service.NewSprintBoardNotifier(boardHub)
	releasePlanExecutor := service.NewReleasePlanExecutor(releasePlanRepo, c.WorkflowRunTrigger)
	sprintWorkItemExecutor := service.NewSprintWorkItemExecutor(repo, sprintRepo, c.WorkflowRunTrigger, pmRouter, boardNotifier)
	getOrSeedSprintProcess := command.NewGetOrSeedSprintProcessHandler(repo, sprintRepo)
	seedProcess := func(ctx context.Context, productID kernel.ProductID, actor kernel.MemberID) (*model.SprintProcess, error) {
		return getOrSeedSprintProcess.Handle(ctx, productID, actor)
	}
	sprintHandlers := producthttp.NewSprintHandlers(
		getOrSeedSprintProcess,
		command.NewUpsertSprintProcessHandler(repo, sprintRepo, getOrSeedSprintProcess),
		command.NewListSprintProcessesHandler(repo, sprintRepo, getOrSeedSprintProcess),
		command.NewCreateSprintProcessHandler(repo, sprintRepo),
		command.NewUpdateSprintProcessByIDHandler(repo, sprintRepo),
		command.NewDeleteSprintProcessHandler(repo, sprintRepo),
		command.NewImportWorkspaceTemplateToProductHandler(repo, sprintRepo),
		command.NewListWorkspaceSprintTemplatesHandler(sprintRepo),
		command.NewCreateWorkspaceSprintTemplateHandler(sprintRepo),
		command.NewDeleteWorkspaceSprintTemplateHandler(sprintRepo),
		command.NewCreateSprintHandler(repo, sprintRepo, getOrSeedSprintProcess),
		command.NewUpdateSprintHandler(repo, sprintRepo, boardNotifier),
		command.NewStartSprintHandler(repo, sprintRepo),
		command.NewCompleteSprintHandler(repo, sprintRepo, releasePlanRepo),
		command.NewArchiveSprintHandler(repo, sprintRepo),
		command.NewCreateWorkItemHandler(repo, sprintRepo, boardNotifier),
		command.NewUpdateWorkItemHandler(repo, sprintRepo),
		command.NewMoveWorkItemHandler(repo, sprintRepo, boardNotifier),
		command.NewDeleteWorkItemHandler(repo, sprintRepo, boardNotifier),
		command.NewExecuteWorkItemHandler(repo, sprintRepo, sprintWorkItemExecutor, boardNotifier),
		query.NewListSprintsHandler(repo, sprintRepo),
		query.NewGetSprintHandler(repo, sprintRepo),
		query.NewGetSprintBoardHandler(repo, sprintRepo),
		query.NewGetWorkItemHandler(repo, sprintRepo),
		query.NewListWorkItemExecutionsHandler(repo, sprintRepo),
		command.NewImportPmSprintHandler(repo, sprintRepo, pmRouter, boardNotifier),
		command.NewLinkPmIssueHandler(repo, sprintRepo, pmRouter),
		command.NewUnlinkPmIssueHandler(repo, sprintRepo, boardNotifier),
		query.NewGetSprintMetricsHandler(repo, sprintRepo),
		query.NewListActiveSprintsHandler(repo, sprintRepo),
	)
	sprintBoardWS := producthttp.NewSprintBoardWSHandler(boardHub)

	releasePlanHandlers := producthttp.NewReleasePlanHandlers(
		command.NewCreateReleasePlanHandler(repo, releasePlanRepo, sprintRepo),
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
		command.NewCreateProductPlanHandler(repo, sprintRepo, seedProcess),
		command.NewCreateProductRequirementHandler(repo, releasePlanRepo),
		command.NewApproveRequirementHandler(repo),
		command.NewRejectRequirementHandler(repo),
		command.NewReleaseRequirementHandler(repo, c.BuildTrigger),
		query.NewListProductsHandler(repo, wsChecker),
		query.NewGetProductHandler(repo, wsChecker),
		query.NewListProductPlansHandler(repo, sprintRepo),
		query.NewListProductRequirementsHandler(repo),
		query.NewListProductProjectsHandler(repo, projectCatalog),
		query.NewGetProductOverviewHandler(repo, projectCatalog, environmentRepo),
		releasePlanHandlers,
		sprintHandlers,
		sprintBoardWS,
		environmentHandlers,
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)

	c.Engine.GET("/api/v1/workspaces/:wsId/sprints/active",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListActiveSprints)
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
	c.EventBus.Subscribe(pwfevent.WorkflowRunCompletedName, subscriber.NewOnWorkflowRunCompleted(releasePlanExecutor, sprintWorkItemExecutor).Handle)
	c.EventBus.Subscribe(deliveryevent.DeployCompletedName, subscriber.NewOnDeployCompleted(environmentRepo).Handle)

	c.Engine.GET("/api/v1/workspaces/:wsId/sprint-process-templates",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListWorkspaceSprintTemplates)
	c.Engine.POST("/api/v1/workspaces/:wsId/sprint-process-templates",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateWorkspaceSprintTemplate)
	c.Engine.DELETE("/api/v1/workspaces/:wsId/sprint-process-templates/:templateId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.DeleteWorkspaceSprintTemplate)
	c.Engine.GET("/api/v1/products/:productId/sprint-process",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetSprintProcess)
	c.Engine.PUT("/api/v1/products/:productId/sprint-process",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpsertSprintProcess)
	c.Engine.GET("/api/v1/products/:productId/sprint-processes",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListSprintProcesses)
	c.Engine.POST("/api/v1/products/:productId/sprint-processes",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateSprintProcess)
	c.Engine.PUT("/api/v1/products/:productId/sprint-processes/:processId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateSprintProcessByID)
	c.Engine.DELETE("/api/v1/products/:productId/sprint-processes/:processId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.DeleteSprintProcess)
	c.Engine.POST("/api/v1/workspaces/:wsId/products/:productId/sprint-processes/import-workspace",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ImportWorkspaceSprintTemplate)
	c.Engine.GET("/api/v1/products/:productId/sprints",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListSprints)
	c.Engine.POST("/api/v1/products/:productId/sprints",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateSprint)
	c.Engine.GET("/api/v1/products/:productId/sprints/:sprintId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetSprint)
	c.Engine.PATCH("/api/v1/products/:productId/sprints/:sprintId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateSprint)
	c.Engine.GET("/api/v1/products/:productId/sprints/:sprintId/board",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetSprintBoard)
	c.Engine.GET("/api/v1/products/:productId/sprints/:sprintId/metrics",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetSprintMetrics)
	c.Engine.POST("/api/v1/products/:productId/sprints/:sprintId/start",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.StartSprint)
	c.Engine.POST("/api/v1/products/:productId/sprints/:sprintId/complete",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CompleteSprint)
	c.Engine.POST("/api/v1/products/:productId/sprints/:sprintId/archive",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ArchiveSprint)
	c.Engine.POST("/api/v1/products/:productId/sprints/:sprintId/work-items",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateWorkItem)
	c.Engine.GET("/api/v1/products/:productId/work-items/:workItemId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetWorkItem)
	c.Engine.PATCH("/api/v1/products/:productId/work-items/:workItemId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateWorkItem)
	c.Engine.DELETE("/api/v1/products/:productId/work-items/:workItemId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.DeleteWorkItem)
	c.Engine.POST("/api/v1/products/:productId/work-items/:workItemId/move",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.MoveWorkItem)
	c.Engine.POST("/api/v1/products/:productId/work-items/:workItemId/execute",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ExecuteWorkItem)
	c.Engine.GET("/api/v1/products/:productId/work-items/:workItemId/executions",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListWorkItemExecutions)
	c.Engine.POST("/api/v1/products/:productId/sprints/:sprintId/import/jira",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ImportJiraSprint)
	c.Engine.POST("/api/v1/products/:productId/sprints/:sprintId/import/pm",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ImportPmSprint)
	c.Engine.POST("/api/v1/products/:productId/work-items/:workItemId/link-jira",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.LinkJiraIssue)
	c.Engine.POST("/api/v1/products/:productId/work-items/:workItemId/link-pm",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.LinkPmIssue)
	c.Engine.DELETE("/api/v1/products/:productId/work-items/:workItemId/link-pm",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UnlinkPmIssue)
	c.Engine.GET("/api/v1/products/:productId/sprints/:sprintId/board/ws",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.SprintBoardWebSocket)

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

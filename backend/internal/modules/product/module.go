package product

import (
	"context"
	"fmt"
	"log"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/subscriber"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/persistence"
	producthttp "github.com/sparkops/sparkops-agent/backend/internal/modules/product/interface/http"
	pwfevent "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/event"
	wspersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
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
	if count, err := service.BackfillReleasePlanVersions(context.Background(), releasePlanRepo, 500); err != nil {
		log.Printf("product: release plan version backfill failed: %v", err)
	} else if count > 0 {
		log.Printf("product: backfilled release plan version snapshots for %d plans", count)
	}
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
	releasePlanCollabHub := service.NewReleasePlanCollabHub()
	releasePlanHookNotifier := service.NewReleasePlanHookNotifier(c.Config.PublicBaseURL())
	releasePlanExecutor := service.NewReleasePlanExecutor(releasePlanRepo, c.WorkflowRunTrigger, releasePlanHookNotifier)
	releasePlanExecutor.UseRuntimeInputs(c.WorkflowRuntimeInputs)
	releasePlanNotifier := service.NewReleasePlanApprovalNotifier(c.Config.PublicBaseURL())
	workflowSnapshotReader := acl.NewWorkflowSnapshotReader(c.DB)
	releasePlanTemplateRepo := persistence.NewReleasePlanTemplateRepository(c.DB)
	service.NewReleasePlanScheduler(releasePlanRepo, releasePlanExecutor, c.Logger).Start(c.Context)
	sprintWorkItemExecutor := service.NewSprintWorkItemExecutor(repo, sprintRepo, c.WorkflowRunTrigger, pmRouter, boardNotifier)
	if c.SprintProcessPort == nil {
		return fmt.Errorf("product module requires sprint process port")
	}
	sprintHandlers := producthttp.NewSprintHandlers(
		command.NewCreateSprintHandler(repo, sprintRepo, c.SprintProcessPort),
		command.NewUpdateSprintHandler(repo, sprintRepo, boardNotifier),
		command.NewStartSprintHandler(repo, sprintRepo),
		command.NewCompleteSprintHandler(repo, sprintRepo, releasePlanRepo),
		command.NewArchiveSprintHandler(repo, sprintRepo),
		command.NewDeleteSprintHandler(repo, sprintRepo, releasePlanRepo),
		command.NewCreateWorkItemHandler(repo, sprintRepo, boardNotifier),
		command.NewUpdateWorkItemHandler(repo, sprintRepo),
		command.NewMoveWorkItemHandler(repo, sprintRepo, sprintWorkItemExecutor, boardNotifier),
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
	releasePlanCollabWS := producthttp.NewReleasePlanCollabWSHandler(releasePlanCollabHub)

	finishPlanning := command.NewFinishReleasePlanPlanningHandler(repo, releasePlanRepo, releasePlanExecutor, releasePlanNotifier, releasePlanHookNotifier, workflowSnapshotReader)
	finishPlanning.UseRuntimeInputs(c.WorkflowRuntimeInputs)
	getReleasePlan := query.NewGetReleasePlanHandlerWithSprint(repo, releasePlanRepo, sprintRepo)
	getReleasePlan.UseRuntimeInputs(c.WorkflowRuntimeInputs)
	releasePlanHandlers := producthttp.NewReleasePlanHandlers(
		command.NewCreateReleasePlanHandler(repo, releasePlanRepo, sprintRepo),
		command.NewUpdateReleasePlanHandler(repo, releasePlanRepo, sprintRepo),
		command.NewCancelReleasePlanHandler(repo, releasePlanRepo),
		command.NewDeleteReleasePlanHandler(repo, releasePlanRepo),
		command.NewReopenReleasePlanPlanningHandler(repo, releasePlanRepo),
		command.NewAddReleasePlanJobHandler(repo, releasePlanRepo),
		command.NewAddReleasePlanJobsFromWorkItemsHandler(repo, releasePlanRepo, sprintRepo),
		command.NewUpdateReleasePlanJobHandler(repo, releasePlanRepo),
		command.NewDeleteReleasePlanJobHandler(repo, releasePlanRepo),
		command.NewReorderReleasePlanJobsHandler(repo, releasePlanRepo),
		finishPlanning,
		command.NewApproveReleasePlanHandler(repo, releasePlanRepo, releasePlanExecutor, releasePlanHookNotifier),
		command.NewRejectReleasePlanHandler(repo, releasePlanRepo),
		command.NewMarkReleasePlanJobDoneHandler(repo, releasePlanRepo, releasePlanExecutor),
		command.NewSkipReleasePlanJobHandler(repo, releasePlanRepo, releasePlanExecutor),
		command.NewExecuteReleasePlanHandler(repo, releasePlanRepo, releasePlanExecutor),
		command.NewRetryReleasePlanJobHandler(repo, releasePlanRepo, releasePlanExecutor),
		command.NewCopyReleasePlanHandler(repo, releasePlanRepo),
		command.NewSaveReleasePlanTemplateHandler(repo, releasePlanRepo, releasePlanTemplateRepo),
		command.NewCreateReleasePlanFromTemplateHandler(repo, releasePlanRepo, releasePlanTemplateRepo, sprintRepo),
		command.NewListReleasePlanTemplatesHandler(releasePlanTemplateRepo),
		command.NewDeleteReleasePlanTemplateHandler(releasePlanTemplateRepo),
		command.NewApproveReleasePlanByTokenHandler(releasePlanRepo, releasePlanExecutor, releasePlanHookNotifier),
		command.NewRejectReleasePlanByTokenHandler(releasePlanRepo),
		command.NewReleasePlanHookCallbackHandler(releasePlanRepo, releasePlanExecutor, releasePlanNotifier, releasePlanHookNotifier),
		query.NewListReleasePlansHandler(repo, releasePlanRepo),
		query.NewListWorkspaceReleasePlansHandler(repo, releasePlanRepo, sprintRepo),
		getReleasePlan,
		query.NewListReleasePlanAuditLogsHandler(repo, releasePlanRepo),
		query.NewListReleasePlanVersionsHandler(repo, releasePlanRepo),
		query.NewGetReleasePlanVersionDiffHandler(repo, releasePlanRepo),
		query.NewListBlockingReleasePlansHandler(releasePlanRepo),
	)

	handler := producthttp.NewHandler(
		command.NewCreateProductHandler(repo, wsChecker),
		command.NewUpdateProductHandler(repo, wsChecker),
		command.NewCreateProductPlanHandler(repo, sprintRepo, c.SprintProcessPort),
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
		releasePlanCollabWS,
		environmentHandlers,
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)

	c.Engine.GET("/api/v1/workspaces/:wsId/sprints/active",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListActiveSprints)
	c.Engine.GET("/api/v1/workspaces/:wsId/release-plans",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListWorkspaceReleasePlans)
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
	c.Engine.GET("/api/v1/products/:productId/release-plans/:planId/audit-logs",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListReleasePlanAuditLogs)
	c.Engine.GET("/api/v1/products/:productId/release-plans/:planId/versions",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListReleasePlanVersions)
	c.Engine.GET("/api/v1/products/:productId/release-plans/:planId/versions/:version/diff",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetReleasePlanVersionDiff)
	c.Engine.PATCH("/api/v1/products/:productId/release-plans/:planId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateReleasePlan)
	c.Engine.DELETE("/api/v1/products/:productId/release-plans/:planId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.DeleteReleasePlan)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/cancel",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CancelReleasePlan)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/reopen-planning",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ReopenReleasePlanPlanning)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/finish-planning",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.FinishReleasePlanPlanning)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/approve",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ApproveReleasePlan)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/reject",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.RejectReleasePlan)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/jobs",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.AddReleasePlanJob)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/jobs/from-work-items",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.AddReleasePlanJobsFromWorkItems)
	c.Engine.PUT("/api/v1/products/:productId/release-plans/:planId/jobs/reorder",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ReorderReleasePlanJobs)
	c.Engine.PATCH("/api/v1/products/:productId/release-plans/:planId/jobs/:jobId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateReleasePlanJob)
	c.Engine.DELETE("/api/v1/products/:productId/release-plans/:planId/jobs/:jobId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.DeleteReleasePlanJob)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/jobs/:jobId/done",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.MarkReleasePlanJobDone)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/jobs/:jobId/skip",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.SkipReleasePlanJob)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/execute",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.ExecuteReleasePlan)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/jobs/:jobId/retry",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.RetryReleasePlanJob)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/copy",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CopyReleasePlan)
	c.Engine.POST("/api/v1/products/:productId/release-plans/:planId/save-template",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.SaveReleasePlanTemplate)
	c.Engine.GET("/api/v1/workspaces/:wsId/release-plan-templates",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListReleasePlanTemplates)
	c.Engine.POST("/api/v1/workspaces/:wsId/release-plan-templates/:templateId/create-plan",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateReleasePlanFromTemplate)
	c.Engine.DELETE("/api/v1/workspaces/:wsId/release-plan-templates/:templateId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.DeleteReleasePlanTemplate)
	c.Engine.GET("/api/v1/public/release-plans/approval/:token/approve", handler.PublicApproveReleasePlanByToken)
	c.Engine.POST("/api/v1/public/release-plans/approval/:token/approve", handler.PublicApproveReleasePlanByToken)
	c.Engine.GET("/api/v1/public/release-plans/approval/:token/reject", handler.PublicRejectReleasePlanByToken)
	c.Engine.POST("/api/v1/public/release-plans/approval/:token/reject", handler.PublicRejectReleasePlanByToken)
	c.Engine.GET("/api/v1/public/release-plans/hook/:token/pass", handler.PublicPassReleasePlanHook)
	c.Engine.POST("/api/v1/public/release-plans/hook/:token/pass", handler.PublicPassReleasePlanHook)
	c.Engine.GET("/api/v1/public/release-plans/hook/:token/fail", handler.PublicFailReleasePlanHook)
	c.Engine.POST("/api/v1/public/release-plans/hook/:token/fail", handler.PublicFailReleasePlanHook)
	c.Engine.GET("/api/v1/products/:productId/release-plans/:planId/collaboration/editors",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListReleasePlanCollabEditors)
	c.Engine.GET("/api/v1/products/:productId/release-plans/:planId/collaboration/ws",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ReleasePlanCollabWebSocket)

	if err := c.EventBus.Subscribe(
		deliveryevent.BuildCompletedName,
		"product.build-completed",
		subscriber.NewOnBuildCompleted(repo).Handle,
	); err != nil {
		return fmt.Errorf("subscribe product build completed: %w", err)
	}
	if err := c.EventBus.Subscribe(
		pwfevent.WorkflowRunCompletedName,
		"product.workflow-run-completed",
		subscriber.NewOnWorkflowRunCompleted(releasePlanExecutor, sprintWorkItemExecutor).Handle,
	); err != nil {
		return fmt.Errorf("subscribe product workflow run completed: %w", err)
	}
	if err := c.EventBus.Subscribe(
		deliveryevent.DeployCompletedName,
		"product.deploy-completed",
		subscriber.NewOnDeployCompleted(environmentRepo).Handle,
	); err != nil {
		return fmt.Errorf("subscribe product deploy completed: %w", err)
	}

	c.Engine.GET("/api/v1/products/:productId/sprints",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListSprints)
	c.Engine.POST("/api/v1/products/:productId/sprints",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateSprint)
	c.Engine.GET("/api/v1/products/:productId/sprints/:sprintId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetSprint)
	c.Engine.PATCH("/api/v1/products/:productId/sprints/:sprintId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateSprint)
	c.Engine.DELETE("/api/v1/products/:productId/sprints/:sprintId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.DeleteSprint)
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

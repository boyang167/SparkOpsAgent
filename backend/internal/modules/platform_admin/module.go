package platform_admin

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/infrastructure/persistence"
	adminhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/platform_admin/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/applog"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/operationlog"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "platform_admin" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("platform_admin module requires database")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("platform_admin module requires permission checker")
	}
	if c.SystemRoleChecker == nil {
		return fmt.Errorf("platform_admin module requires system role checker")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("platform_admin migrate: %w", err)
	}

	repo := persistence.NewAdminRepository(c.DB)
	createSyncJob := command.NewCreateSyncJobHandler(repo)
	handler := adminhttp.NewHandler(
		query.NewGetSysConfigHandler(repo),
		command.NewUpdateSysConfigHandler(repo),
		query.NewListClustersHandler(repo),
		command.NewCreateClusterHandler(repo),
		query.NewListSyncJobsHandler(repo),
		createSyncJob,
		query.NewGetDashboardHandler(repo),
		query.NewGetStatisticsHandler(repo),
		query.NewGetMonitorOverviewHandler(repo),
		query.NewListDeploymentStatsHandler(repo),
		query.NewListToolsHandler(),
		query.NewListProclaimsHandler(repo),
		command.NewCreateProclaimHandler(repo),
		command.NewCloneToolHandler(),
		query.NewGetWorkspaceEntitlementsHandler(c.DB),
	)
	opStore := operationlog.NewStore(c.DB)
	opHandler := adminhttp.NewOperationRecordsHandler(opStore)
	appLogStore := applog.NewStore(c.DB)
	obsHandler := adminhttp.NewObservabilityHandler(appLogStore, opStore, c.Config)

	auth := authn.RequireAuth(m.tokens)
	superAdmin := authz.RequireSuperAdmin(c.SystemRoleChecker)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)
	govAdmin := authz.RequirePermission(c.PermissionChecker, authz.PermGovernanceAdmin)
	projectRead := authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead)

	c.Engine.GET("/api/v1/admin/sysconfig", auth, superAdmin, handler.GetSysConfig)
	c.Engine.PATCH("/api/v1/admin/sysconfig", auth, superAdmin, handler.UpdateSysConfig)
	c.Engine.GET("/api/v1/admin/clusters", auth, superAdmin, handler.ListClusters)
	c.Engine.POST("/api/v1/admin/clusters", auth, superAdmin, handler.CreateCluster)
	c.Engine.GET("/api/v1/admin/sync/jobs", auth, superAdmin, handler.ListSyncJobs)
	c.Engine.POST("/api/v1/admin/sync/jobs", auth, superAdmin, handler.CreateSyncJob)
	c.Engine.GET("/api/v1/admin/tools", auth, superAdmin, handler.ListTools)
	c.Engine.GET("/api/v1/admin/proclaims", auth, superAdmin, handler.ListProclaims)
	c.Engine.POST("/api/v1/admin/proclaims", auth, superAdmin, handler.CreateProclaim)
	c.Engine.POST("/api/v1/admin/tools/clone", auth, superAdmin, handler.CloneTool)
	c.Engine.GET("/api/v1/admin/server/status", auth, superAdmin, handler.GetServerStatus)

	c.Engine.GET("/api/v1/workspaces/:wsId/entitlements",
		auth, ws, wsMember, handler.GetWorkspaceEntitlements)
	c.Engine.GET("/api/v1/workspaces/:wsId/dashboard",
		auth, ws, wsMember, projectRead, handler.GetDashboard)
	c.Engine.GET("/api/v1/workspaces/:wsId/statistics",
		auth, ws, wsMember, projectRead, handler.GetStatistics)
	c.Engine.GET("/api/v1/workspaces/:wsId/monitor/overview",
		auth, ws, wsMember, projectRead, handler.GetMonitorOverview)
	c.Engine.GET("/api/v1/workspaces/:wsId/statistics/deployments",
		auth, ws, wsMember, projectRead, handler.ListDeploymentStats)
	c.Engine.GET("/api/v1/workspaces/:wsId/operation-records",
		auth, ws, wsMember, govAdmin, opHandler.List)
	c.Engine.DELETE("/api/v1/workspaces/:wsId/operation-records/:recordId",
		auth, ws, wsMember, govAdmin, opHandler.Delete)
	c.Engine.POST("/api/v1/workspaces/:wsId/operation-records/batch-delete",
		auth, ws, wsMember, govAdmin, opHandler.BatchDelete)
	c.Engine.GET("/api/v1/workspaces/:wsId/monitor/server/status",
		auth, ws, wsMember, govAdmin, handler.GetServerStatus)
	c.Engine.GET("/api/v1/workspaces/:wsId/observability/logs",
		auth, ws, wsMember, govAdmin, obsHandler.ListLogs)
	c.Engine.GET("/api/v1/workspaces/:wsId/observability/log-files",
		auth, ws, wsMember, govAdmin, obsHandler.ListLogFiles)
	c.Engine.GET("/api/v1/workspaces/:wsId/observability/log-files/:fileName/tail",
		auth, ws, wsMember, govAdmin, obsHandler.TailLogFile)
	c.Engine.GET("/api/v1/workspaces/:wsId/observability/traces/:traceId",
		auth, ws, wsMember, govAdmin, obsHandler.GetTrace)
	c.Engine.GET("/api/v1/workspaces/:wsId/observability/metrics/summary",
		auth, ws, wsMember, govAdmin, obsHandler.MetricsSummary)

	c.SyncJobCreator = &syncJobCreatorPort{handler: createSyncJob}

	return nil
}

type syncJobCreatorPort struct {
	handler *command.CreateSyncJobHandler
}

func (p *syncJobCreatorPort) CreateSyncJob(
	ctx context.Context,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
	name, version, sourceCluster, targetCluster, syncResources string,
) (kernel.SyncJobID, error) {
	job, err := p.handler.Handle(ctx, memberID, wsID, name, version, sourceCluster, targetCluster, syncResources)
	if err != nil {
		return kernel.SyncJobID{}, err
	}
	return job.ID, nil
}

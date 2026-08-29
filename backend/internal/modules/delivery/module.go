package delivery

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/application/query"
	deploycommand "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/command"
	deployquery "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/query"
	deployservice "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/service"
	deploysubscriber "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/subscriber"
	deployacl "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/acl"
	deployconsumer "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/consumer"
	deploypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/persistence"
	deploystream "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/stream"
	deployhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/interface/http"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	deliveryinfra "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/cronjob"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/gitlab"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/runner"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/stream"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/webhook"
	deliveryhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"go.uber.org/zap"
)

type Module struct {
	tokens *authn.TokenProvider
}

func New(tokens *authn.TokenProvider) *Module {
	return &Module{tokens: tokens}
}

func (m *Module) Name() string { return "delivery" }

func (m *Module) Register(c *appctx.Container) error {
	if c.DB == nil {
		return fmt.Errorf("delivery module requires database")
	}
	if c.PermissionChecker == nil {
		return fmt.Errorf("delivery module requires permission checker")
	}
	if c.Context == nil {
		return fmt.Errorf("delivery module requires lifecycle context")
	}
	if err := persistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("delivery migrate: %w", err)
	}
	if err := deploypersistence.AutoMigrate(c.DB); err != nil {
		return fmt.Errorf("delivery deployment migrate: %w", err)
	}

	repo := persistence.NewBuildRepository(c.DB)
	cronRepo := persistence.NewCronJobRepository(c.DB)
	checker := acl.NewWorkspaceAppACL(c.DB)
	executor, err := deliveryinfra.NewJenkinsExecutor(c.Config)
	if err != nil {
		return fmt.Errorf("delivery jenkins: %w", err)
	}
	hub := stream.NewHub()
	registry := runner.NewRegistry()
	abortBuild := command.NewAbortBuildHandler(repo, checker, registry, hub)
	trigger := command.NewTriggerBuildHandler(repo, checker, executor, c.EventBus, hub, registry, c.Context, func(err error) {
		if c.Logger != nil {
			c.Logger.Error("asynchronous build failed", zap.Error(err))
		}
	})
	if err := webhook.NewBuildConsumer(c.Logger, trigger).Register(context.Background(), c.MessageQueue); err != nil {
		return fmt.Errorf("delivery webhook consumer: %w", err)
	}
	cronjob.NewScheduler(cronRepo, trigger, c.Logger).Start(context.Background())

	gitClient := gitlab.NewFromConfig(c.Config.GitLab)
	branches := gitlab.NewListGitBranchesHandler(checker, gitlab.NewBranchLister(gitClient))
	handler := deliveryhttp.NewHandler(
		trigger,
		abortBuild,
		command.NewRollbackBuildHandler(repo, checker, executor, c.EventBus, hub, registry, c.Context),
		query.NewListBuildsHandler(repo, checker),
		query.NewGetBuildHandler(repo, checker),
		query.NewStreamBuildLogsHandler(repo, checker, hub),
		branches,
		query.NewListCronJobsHandler(cronRepo, checker),
		command.NewCreateCronJobHandler(cronRepo, checker),
		command.NewDeleteCronJobHandler(cronRepo, checker),
	)
	public := deliveryhttp.NewPublicHandler(c.MessageQueue, c.Config.Webhook.Secret)
	secretBox, err := crypto.NewSecretBoxFromConfig(c.Config)
	if err != nil {
		return fmt.Errorf("delivery deployment secrets: %w", err)
	}
	deployHub := deploystream.NewHub()
	deployRepo := deploypersistence.NewDeploymentRunRepositoryWithObserver(c.DB, deployHub, func(err error) {
		if c.Logger != nil {
			c.Logger.Error("deployment event observer failed", zap.Error(err))
		}
	})
	deployReader := deployacl.NewWorkspaceAppACL(c.DB, secretBox)
	if c.ManifestDeployer == nil {
		return fmt.Errorf("delivery module requires manifest deployer")
	}
	deployPublisher := deployconsumer.NewPublisher(c.MessageQueue)
	deployOperations := runner.NewRegistry()
	deployOrchestrator := deployservice.NewOrchestrator(
		deployRepo,
		repo,
		deployReader,
		deployservice.NewPrepareService(executor, deployacl.NewWorkspacePipelineRenderer(c.DB)),
		trigger,
		deployservice.NewManifestRenderer(),
		c.ManifestDeployer,
		deployPublisher,
		c.Config.Deploy.RolloutTimeout,
		c.Config.Deploy.JenkinsTimeout,
		deployOperations,
	)
	deployConsumer, err := deployconsumer.New(deployRepo, deployOrchestrator)
	if err != nil {
		return fmt.Errorf("delivery deployment consumer: %w", err)
	}
	if err := deployConsumer.Register(c.Context, c.MessageQueue); err != nil {
		return fmt.Errorf("delivery deployment consumer: %w", err)
	}
	onBuildCompleted := deploysubscriber.NewOnBuildCompleted(deployRepo, deployPublisher)
	c.EventBus.Subscribe(deliveryevent.BuildCompletedName, onBuildCompleted.Handle)
	deployservice.NewReconciler(deployRepo, deployPublisher, func(err error) {
		if c.Logger != nil {
			c.Logger.Error("deployment reconciliation failed", zap.Error(err))
		}
	}).Start(c.Context, c.Config.Deploy.ReconcileInterval)
	deployHandler := deployhttp.NewHandler(
		deploycommand.NewCreateDeploymentHandler(deployRepo, deployReader, c.MessageQueue, nil),
		deploycommand.NewAbortDeploymentHandler(deployRepo, nil, deploycommand.AbortDependencies{
			Builds: abortBuild,
			Runs:   deployOperations,
		}),
		deployquery.NewListDeploymentsHandler(deployRepo),
		deployquery.NewGetDeploymentHandler(deployRepo),
		deployquery.NewDeploymentOptionsHandler(deployReader),
		deployquery.NewStreamDeploymentHandler(deployRepo, deployHub),
	)

	auth := authn.RequireAuth(m.tokens)
	ws := httpx.RequireWorkspace()
	wsMember := authz.RequireWorkspaceMember(c.WorkspaceMemberChecker)

	c.Engine.POST("/api/v1/projects/:projectId/builds",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.TriggerBuild)
	c.Engine.GET("/api/v1/projects/:projectId/builds",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListBuilds)
	c.Engine.GET("/api/v1/builds/:buildId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.GetBuild)
	c.Engine.GET("/api/v1/builds/:buildId/stream",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.StreamBuildLogs)
	c.Engine.POST("/api/v1/builds/:buildId/abort",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.AbortBuild)
	c.Engine.POST("/api/v1/builds/:buildId/rollback",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.RollbackBuild)
	c.Engine.GET("/api/v1/projects/:projectId/git/branches",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListGitBranches)
	c.Engine.GET("/api/v1/projects/:projectId/cronjobs",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListCronJobs)
	c.Engine.POST("/api/v1/projects/:projectId/cronjobs",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateCronJob)
	c.Engine.DELETE("/api/v1/projects/:projectId/cronjobs/:cronJobId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.DeleteCronJob)

	c.Engine.POST("/api/v1/public/builds/webhook", public.BuildWebhook)
	c.Engine.POST("/api/v1/projects/:projectId/deployments",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectOperate), deployHandler.Create)
	c.Engine.GET("/api/v1/projects/:projectId/deployments",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), deployHandler.List)
	c.Engine.GET("/api/v1/projects/:projectId/deployments/options",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), deployHandler.Options)
	c.Engine.GET("/api/v1/deployments/:runId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), deployHandler.Get)
	c.Engine.GET("/api/v1/deployments/:runId/stream",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), deployHandler.Stream)
	c.Engine.POST("/api/v1/deployments/:runId/abort",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectOperate), deployHandler.Abort)

	c.BuildTrigger = &buildTriggerPort{handler: trigger}

	return nil
}

type buildTriggerPort struct {
	handler *command.TriggerBuildHandler
}

func (p *buildTriggerPort) TriggerAutomated(ctx context.Context, projectID kernel.ProjectID, branch string) (kernel.BuildID, error) {
	build, err := p.handler.TriggerAutomated(ctx, projectID, branch)
	if err != nil {
		return kernel.BuildID{}, err
	}
	return build.ID, nil
}

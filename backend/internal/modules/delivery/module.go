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
	deployrunmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployacl "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/acl"
	deployconsumer "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/consumer"
	deploypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/persistence"
	deploystream "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/stream"
	deployhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/interface/http"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	deliveryinfra "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/cronjob"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/gitlab"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/outbound"
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
	"github.com/sparkops/sparkops-agent/backend/internal/shared/integration"
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
	hookRepo := persistence.NewWebhookRepository(c.DB)
	checker := acl.NewWorkspaceAppACL(c.DB)
	secretBox, err := crypto.NewSecretBoxFromConfig(c.Config)
	if err != nil {
		return fmt.Errorf("delivery secrets: %w", err)
	}
	jenkinsProvider, err := deliveryinfra.NewJenkinsExecutorProvider(c.Config, c.DB, secretBox)
	if err != nil {
		return fmt.Errorf("delivery jenkins: %w", err)
	}
	hub := stream.NewHub()
	registry := runner.NewRegistry()
	abortBuild := command.NewAbortBuildHandler(repo, checker, registry, hub)
	trigger := command.NewTriggerBuildHandler(repo, checker, jenkinsProvider, c.EventBus, hub, registry, c.Context, func(err error) {
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
		command.NewRollbackBuildHandler(repo, checker, jenkinsProvider, c.EventBus, hub, registry, c.Context),
		query.NewListBuildsHandler(repo, checker),
		query.NewGetBuildHandler(repo, checker),
		query.NewStreamBuildLogsHandler(repo, checker, hub),
		branches,
		query.NewListCronJobsHandler(cronRepo, checker),
		command.NewCreateCronJobHandler(cronRepo, checker),
		command.NewDeleteCronJobHandler(cronRepo, checker),
		query.NewListWebhooksHandler(hookRepo, checker),
		command.NewCreateWebhookHandler(hookRepo, checker),
		command.NewUpdateWebhookHandler(hookRepo, checker),
		command.NewDeleteWebhookHandler(hookRepo, checker),
	)
	public := deliveryhttp.NewPublicHandler(c.MessageQueue, c.Config.Webhook.Secret)
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
		deployservice.NewPrepareService(c.Config.Jenkins.JobSuffix, deployacl.NewWorkspacePipelineRenderer(c.DB)),
		trigger,
		deployservice.NewManifestRenderer(),
		c.ManifestDeployer,
		deployPublisher,
		c.Config.Deploy.RolloutTimeout,
		c.Config.Deploy.JenkinsTimeout,
		deployOperations,
	)
	deployOrchestrator.SetTerminalHook(func(ctx context.Context, run *deployrunmodel.DeploymentRun) {
		if run == nil {
			return
		}
		if err := c.EventBus.Publish(ctx, deliveryevent.NewDeployCompleted(
			run.ID, run.ProjectID, run.WorkspaceID, run.EnvironmentID, "", string(run.Status), string(run.RunType), run.ImageTag,
		)); err != nil && c.Logger != nil {
			c.Logger.Warn("publish deploy completed failed", zap.Error(err))
		}
	})
	deployConsumer, err := deployconsumer.New(deployRepo, deployOrchestrator)
	if err != nil {
		return fmt.Errorf("delivery deployment consumer: %w", err)
	}
	if err := deployConsumer.Register(c.Context, c.MessageQueue); err != nil {
		return fmt.Errorf("delivery deployment consumer: %w", err)
	}
	onBuildCompleted := deploysubscriber.NewOnBuildCompleted(deployRepo, deployPublisher)
	c.EventBus.Subscribe(deliveryevent.BuildCompletedName, onBuildCompleted.Handle)
	hookDispatcher := outbound.NewDispatcher(hookRepo, c.Logger)
	c.EventBus.Subscribe(deliveryevent.BuildCompletedName, hookDispatcher.Handle)
	c.EventBus.Subscribe(deliveryevent.DeployCompletedName, hookDispatcher.Handle)
	credentialSync := deployservice.NewCredentialSyncService(
		deployReader,
		jenkinsProvider,
		c.Config.Deploy.JenkinsTimeout,
		c.Logger,
	)
	c.EventBus.Subscribe(integration.EventSecretUpdated, deploysubscriber.NewOnSecretUpdated(credentialSync, c.Logger, c.Config.Deploy.JenkinsTimeout).Handle)
	deployservice.NewReconciler(deployRepo, deployPublisher, func(err error) {
		if c.Logger != nil {
			c.Logger.Error("deployment reconciliation failed", zap.Error(err))
		}
	}).Start(c.Context, c.Config.Deploy.ReconcileInterval)
	envResolver := deployacl.NewEnvironmentResolver(c.DB)
	createDeployment := deploycommand.NewCreateDeploymentHandler(deployRepo, deployReader, c.MessageQueue, nil, envResolver)
	deployHandler := deployhttp.NewHandler(
		createDeployment,
		deploycommand.NewRollbackDeploymentHandler(deployRepo, c.MessageQueue, nil),
		deploycommand.NewAbortDeploymentHandler(deployRepo, nil, deploycommand.AbortDependencies{
			Builds: abortBuild,
			Runs:   deployOperations,
		}),
		deployquery.NewListDeploymentsHandler(deployRepo),
		deployquery.NewGetDeploymentHandler(deployRepo),
		deployquery.NewDeploymentOptionsHandler(deployReader),
		deployquery.NewRollbackPreviewHandler(deployRepo),
		deployquery.NewSyncPreviewHandler(deployReader),
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
	c.Engine.GET("/api/v1/projects/:projectId/webhooks",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), handler.ListWebhooks)
	c.Engine.POST("/api/v1/projects/:projectId/webhooks",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.CreateWebhook)
	c.Engine.PUT("/api/v1/projects/:projectId/webhooks/:webhookId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.UpdateWebhook)
	c.Engine.DELETE("/api/v1/projects/:projectId/webhooks/:webhookId",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectWrite), handler.DeleteWebhook)

	c.Engine.POST("/api/v1/public/builds/webhook", public.BuildWebhook)
	c.Engine.POST("/api/v1/projects/:projectId/deployments",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectOperate), deployHandler.Create)
	c.Engine.POST("/api/v1/projects/:projectId/deployments/rollback",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectOperate), deployHandler.Rollback)
	c.Engine.GET("/api/v1/projects/:projectId/deployments/rollback-preview",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), deployHandler.RollbackPreview)
	c.Engine.GET("/api/v1/projects/:projectId/deployments/sync-preview",
		auth, ws, wsMember, authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead), deployHandler.SyncPreview)
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
	c.DeploymentTrigger = &deploymentTriggerPort{create: createDeployment, checker: checker}

	return nil
}

type buildTriggerPort struct {
	handler *command.TriggerBuildHandler
}

func (p *buildTriggerPort) TriggerAutomated(ctx context.Context, projectID kernel.ProjectID, branch string) (kernel.BuildID, error) {
	return p.TriggerAutomatedBuild(ctx, appctx.AutomatedBuildRequest{ProjectID: projectID, Branch: branch})
}

func (p *buildTriggerPort) TriggerAutomatedBuild(ctx context.Context, req appctx.AutomatedBuildRequest) (kernel.BuildID, error) {
	build, err := p.handler.TriggerAutomatedBuild(ctx, req.ProjectID, req.Branch, req.ImageRepository)
	if err != nil {
		return kernel.BuildID{}, err
	}
	return build.ID, nil
}

type deploymentTriggerPort struct {
	create  *deploycommand.CreateDeploymentHandler
	checker deliveryport.ProjectChecker
}

func (p *deploymentTriggerPort) TriggerForEnvironment(ctx context.Context, req appctx.WorkflowDeploymentRequest) (kernel.DeploymentRunID, error) {
	auto, err := p.checker.ProjectAutomationContext(ctx, req.ProjectID)
	if err != nil {
		return kernel.DeploymentRunID{}, err
	}
	run, err := p.create.Handle(ctx, auto.WorkspaceID, auto.MemberID, req.ProjectID, deploycommand.CreateInput{
		EnvironmentID: req.EnvironmentID.String(),
		GitRefType:    req.GitRefType,
		GitRefValue:   req.GitRefValue,
		ImageTag:      req.ImageTag,
	})
	if err != nil {
		return kernel.DeploymentRunID{}, err
	}
	return run.ID, nil
}

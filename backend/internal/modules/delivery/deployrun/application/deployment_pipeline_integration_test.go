package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	buildcommand "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/subscriber"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	deployacl "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/consumer"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/persistence"
	deploystream "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/stream"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/jenkins"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/runner"
	buildstream "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/stream"
	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	runtimek8s "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	workspacemodel "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type pipelineMode string

const (
	pipelineModeSuccess       pipelineMode = "success"
	pipelineModeCIFailure     pipelineMode = "ci-failure"
	pipelineModeMissingHarbor pipelineMode = "missing-harbor"
	pipelineModeBlockedBuild  pipelineMode = "blocked-build"
	pipelineModeReconcile     pipelineMode = "reconcile"
)

type acceptanceBuildRepo struct {
	mu       sync.Mutex
	builds   map[kernel.BuildID]*buildmodel.Build
	terminal chan struct{}
	once     sync.Once
}

func (r *acceptanceBuildRepo) Save(_ context.Context, build *buildmodel.Build) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builds[build.ID] = build
	return nil
}

func (r *acceptanceBuildRepo) Update(_ context.Context, build *buildmodel.Build) error {
	r.mu.Lock()
	r.builds[build.ID] = build
	terminal := !build.IsRunning()
	r.mu.Unlock()
	if terminal {
		r.once.Do(func() { close(r.terminal) })
	}
	return nil
}

func (r *acceptanceBuildRepo) FindByID(_ context.Context, id kernel.BuildID) (*buildmodel.Build, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	build := r.builds[id]
	if build == nil {
		return nil, errors.New("build not found")
	}
	return build, nil
}

func (r *acceptanceBuildRepo) ListByProject(context.Context, kernel.ProjectID, int) ([]*buildmodel.Build, error) {
	return nil, nil
}

func (r *acceptanceBuildRepo) HasRunningBuild(_ context.Context, projectID kernel.ProjectID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, build := range r.builds {
		if build.ProjectID == projectID && build.IsRunning() {
			return true, nil
		}
	}
	return false, nil
}

func (r *acceptanceBuildRepo) NextBuildNumber(context.Context, kernel.ProjectID) (int, error) {
	return 1, nil
}

type acceptanceProject struct {
	context deployport.ProjectDeploymentContext
}

func (p *acceptanceProject) ProjectInWorkspace(_ context.Context, projectID kernel.ProjectID, wsID kernel.WorkspaceID) (bool, error) {
	return projectID == p.context.ProjectID && wsID == p.context.WorkspaceID, nil
}

func (*acceptanceProject) ProjectIsActive(context.Context, kernel.ProjectID) (bool, error) {
	return true, nil
}

func (p *acceptanceProject) ProjectName(context.Context, kernel.ProjectID) (string, error) {
	return p.context.Name, nil
}

func (p *acceptanceProject) ProjectNamespace(context.Context, kernel.ProjectID) (string, error) {
	return p.context.Namespace, nil
}

func (p *acceptanceProject) ProjectAutomationContext(context.Context, kernel.ProjectID) (deliveryport.ProjectAutomationContext, error) {
	return deliveryport.ProjectAutomationContext{WorkspaceID: p.context.WorkspaceID}, nil
}

type acceptanceQueue struct {
	inner messaging.Queue
	mu    sync.Mutex
	count int
}

func (q *acceptanceQueue) Publish(ctx context.Context, topic string, payload []byte) error {
	q.mu.Lock()
	q.count++
	q.mu.Unlock()
	return q.inner.Publish(ctx, topic, payload)
}

func (q *acceptanceQueue) Subscribe(
	ctx context.Context,
	topic string,
	handler func(context.Context, []byte) error,
) error {
	return q.inner.Subscribe(ctx, topic, handler)
}

func (q *acceptanceQueue) Close() error {
	return q.inner.Close()
}

func (q *acceptanceQueue) publishCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.count
}

type acceptancePipelineRenderer struct{}

func (acceptancePipelineRenderer) Render(context.Context, deployport.PipelineRenderRequest) (string, error) {
	return "pipeline { stages { stage('build') { steps { echo 'mock' } } } }", nil
}

type acceptanceJenkins struct {
	*jenkins.MockExecutor
	mode        pipelineMode
	release     chan struct{}
	releaseOnce sync.Once
}

func (e *acceptanceJenkins) RunStreaming(
	ctx context.Context,
	buildID kernel.BuildID,
	req deliveryport.BuildRequest,
	hub deliveryport.LogPublisher,
) (string, error) {
	switch e.mode {
	case pipelineModeCIFailure:
		e.LastBuildRequest = req
		return "mock ci failed", errors.New("mock ci failed")
	case pipelineModeBlockedBuild:
		e.LastBuildRequest = req
		select {
		case <-e.release:
			return "mock build released", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	default:
		return e.MockExecutor.RunStreaming(ctx, buildID, req, hub)
	}
}

func (e *acceptanceJenkins) unblock() {
	e.releaseOnce.Do(func() { close(e.release) })
}

type acceptanceRuntime struct {
	mock               *runtimek8s.MockManifestDeployer
	applyCalls         int
	lastDeploymentYAML string
	lastKubeconfig     string
}

func (r *acceptanceRuntime) Deploy(ctx context.Context, target runtimeport.DeploymentTarget, manifests []runtimeport.Manifest) error {
	if err := r.mock.Deploy(ctx, target, manifests); err != nil {
		return err
	}
	r.applyCalls++
	r.lastKubeconfig = target.Kubeconfig
	for _, manifest := range manifests {
		if jsonKind(manifest.YAML) == "Deployment" {
			r.lastDeploymentYAML = manifest.YAML
		}
	}
	return nil
}

func (r *acceptanceRuntime) RolloutRestart(ctx context.Context, target runtimeport.DeploymentTarget) error {
	return r.mock.RolloutRestart(ctx, target)
}

type pipelineFixture struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	wsID                  kernel.WorkspaceID
	memberID              kernel.MemberID
	projectID             kernel.ProjectID
	unauthorizedClusterID kernel.ClusterInfoID
	repo                  *persistence.DeploymentRunRepository
	builds                *acceptanceBuildRepo
	project               *acceptanceProject
	queue                 *acceptanceQueue
	jenkins               *acceptanceJenkins
	runtime               *acceptanceRuntime
	createHandler         *command.CreateDeploymentHandler
	abort                 *command.AbortDeploymentHandler
	stream                *query.StreamDeploymentHandler
	reconciler            *service.Reconciler
}

func newPipelineFixture(t *testing.T, mode pipelineMode) *pipelineFixture {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	db := acceptanceDB(t)
	wsID, memberID, projectID := kernel.NewWorkspaceID(), kernel.NewMemberID(), kernel.NewProjectID()
	clusterID, unauthorizedClusterID := kernel.NewClusterInfoID(), kernel.NewClusterInfoID()
	box, err := crypto.NewSecretBox([]byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	seedAcceptanceDeploymentBoundary(
		t, db, box, mode, wsID, memberID, projectID, clusterID, unauthorizedClusterID,
	)
	reader := deployacl.NewWorkspaceAppACL(db, box)
	project := &acceptanceProject{
		context: deployport.ProjectDeploymentContext{
			WorkspaceID: wsID, ProjectID: projectID, Name: "orders", Namespace: "team",
		},
	}
	deployHub := deploystream.NewHub()
	repo := persistence.NewDeploymentRunRepository(db, deployHub)
	builds := &acceptanceBuildRepo{builds: make(map[kernel.BuildID]*buildmodel.Build), terminal: make(chan struct{})}
	queue := &acceptanceQueue{inner: messaging.NewNoopQueue()}
	t.Cleanup(func() { require.NoError(t, queue.Close()) })
	jenkinsMock := &acceptanceJenkins{
		MockExecutor: jenkins.NewMockExecutor(), mode: mode, release: make(chan struct{}),
	}
	t.Cleanup(jenkinsMock.unblock)
	bus := eventbus.NewInMemory()
	buildRegistry := runner.NewRegistry()
	buildHub := buildstream.NewHub()
	trigger := buildcommand.NewTriggerBuildHandler(
		builds, project, jenkins.NewStaticProvider(jenkinsMock), bus, buildHub, buildRegistry, ctx,
		postgres.NewDirectTransactor(),
	)
	runtime := &acceptanceRuntime{mock: runtimek8s.NewMockManifestDeployer()}
	publisher := consumer.NewPublisher(queue)
	operations := runner.NewRegistry()
	prepare := service.NewPrepareService("-build", acceptancePipelineRenderer{}).
		UseJenkinsFactory(func(deployport.DeploymentSecret) (deliveryport.JenkinsExecutor, error) {
			return jenkinsMock, nil
		}).
		UseHarborEnsurer(func(context.Context, deployport.DeploymentSecret, string) error { return nil })
	orchestrator := service.NewOrchestrator(
		repo, builds, reader,
		prepare,
		trigger, service.NewManifestRenderer(), runtime, publisher,
		time.Minute, time.Minute, operations,
	)
	deployConsumer, err := consumer.New(repo, orchestrator)
	require.NoError(t, err)
	require.NoError(t, deployConsumer.Register(ctx, queue))
	if mode != pipelineModeReconcile {
		require.NoError(t, bus.Subscribe(
			deliveryevent.BuildCompletedName,
			"test.deployment-pipeline.build-completed",
			subscriber.NewOnBuildCompleted(repo, publisher).Handle,
		))
	}
	return &pipelineFixture{
		ctx: ctx, cancel: cancel, wsID: wsID, memberID: memberID, projectID: projectID,
		unauthorizedClusterID: unauthorizedClusterID,
		repo:                  repo, builds: builds, project: project, queue: queue, jenkins: jenkinsMock, runtime: runtime,
		createHandler: command.NewCreateDeploymentHandler(repo, reader, queue, nil),
		abort: command.NewAbortDeploymentHandler(repo, nil, command.AbortDependencies{
			Builds: buildcommand.NewAbortBuildHandler(builds, project, buildRegistry, buildHub),
			Runs:   operations,
		}),
		stream:     query.NewStreamDeploymentHandler(repo, deployHub),
		reconciler: service.NewReconciler(repo, publisher),
	}
}

func acceptanceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// Every new connection to ":memory:" opens an empty database without the attached schemas.
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS delivery").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS workspace_app").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform_admin").Error)
	require.NoError(t, db.Exec(`CREATE TABLE delivery.deployment_runs (
		id TEXT PRIMARY KEY, workspace_id TEXT, project_id TEXT, cluster_id TEXT,
		git_ref_type TEXT, git_ref_value TEXT, image_tag TEXT, image_repository TEXT NOT NULL DEFAULT '', cross_cluster INTEGER NOT NULL DEFAULT 0,
		sync_manifests TEXT NOT NULL DEFAULT '[]',
		status TEXT, build_id TEXT,
		error_message TEXT, created_by TEXT, created_at DATETIME, started_at DATETIME,
		finished_at DATETIME, current_stage TEXT, run_type TEXT NOT NULL DEFAULT 'normal',
		rollback_of_run_id TEXT, version INTEGER NOT NULL DEFAULT 0,
		attempt_count INTEGER NOT NULL DEFAULT 0, lease_owner TEXT, lease_expires_at DATETIME,
		environment_id TEXT, deploy_namespace TEXT NOT NULL DEFAULT '', project_version_id TEXT, undeploy_options TEXT,
		archive_generation TEXT)`).Error)
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX delivery.uq_deployment_runs_active_project
		ON deployment_runs(project_id) WHERE status IN ('queued','preparing','building','deploying')`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE delivery.deployment_run_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT, at DATETIME,
		status TEXT NOT NULL, stage TEXT, message TEXT)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE workspace_app.projects (
		id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, display_name TEXT, namespace TEXT,
		language TEXT, git_url TEXT, git_ref_type TEXT, git_ref_value TEXT,
		dockerfile_path TEXT, template_id TEXT, secret_id TEXT, review_status TEXT,
		resources BLOB, default_cluster_id TEXT, jenkins_secret_id TEXT,
		kubernetes_secret_id TEXT)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE workspace_app.secrets (
		id TEXT PRIMARY KEY, workspace_id TEXT, service_type TEXT, service_url TEXT,
		username TEXT, credential_blob TEXT, status TEXT)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE platform_admin.cluster_info (
		id TEXT PRIMARY KEY, cluster_name TEXT, server TEXT,
		is_build_env BOOLEAN, priority_id INTEGER)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE platform_admin.workspace_clusters (
		workspace_id TEXT, cluster_id TEXT, created_by TEXT, created_at DATETIME,
		PRIMARY KEY (workspace_id, cluster_id))`).Error)
	return db
}

func seedAcceptanceDeploymentBoundary(
	t *testing.T,
	db *gorm.DB,
	box *crypto.SecretBox,
	mode pipelineMode,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	clusterID kernel.ClusterInfoID,
	unauthorizedClusterID kernel.ClusterInfoID,
) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO platform_admin.cluster_info(id, cluster_name, server, is_build_env, priority_id)
		 VALUES (?, 'authorized', 'https://cluster.test.invalid', 0, 1),
		        (?, 'unauthorized', 'https://other-cluster.test.invalid', 0, 2)`,
		clusterID.String(), unauthorizedClusterID.String(),
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO platform_admin.workspace_clusters(workspace_id, cluster_id, created_by, created_at)
		 VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		wsID.String(), clusterID.String(), memberID.String(),
	).Error)

	secretIDs := map[string]kernel.SecretID{}
	for _, serviceType := range []string{"GitLab", "Harbor", "Jenkins", "Kubernetes"} {
		secretID := kernel.NewSecretID()
		secretIDs[serviceType] = secretID
		credentials := workspacemodel.SecretCredentials{Token: "fixture-value"}
		serviceURL := "https://" + strings.ToLower(serviceType) + ".test.invalid"
		if serviceType == "Harbor" {
			serviceURL = "https://registry.test.invalid"
			credentials = workspacemodel.SecretCredentials{Password: "fixture-value"}
		}
		if serviceType == "Kubernetes" {
			credentials = workspacemodel.SecretCredentials{
				ExtraConfig: "apiVersion: v1\nclusters: []\n",
			}
		}
		raw, err := json.Marshal(credentials)
		require.NoError(t, err)
		encrypted, err := box.Encrypt(string(raw))
		require.NoError(t, err)
		require.NotContains(t, encrypted, "fixture-value")
		require.NoError(t, db.Exec(
			`INSERT INTO workspace_app.secrets(
				id, workspace_id, service_type, service_url, username, credential_blob, status)
			 VALUES (?, ?, ?, ?, 'fixture-user', ?, 'active')`,
			secretID.String(), wsID.String(), serviceType, serviceURL, encrypted,
		).Error)
	}
	resources := workspacemodel.ProjectResources{}
	if mode != pipelineModeMissingHarbor {
		resources.RegistrySecretID = secretIDs["Harbor"].String()
	}
	rawResources, err := json.Marshal(resources)
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		`INSERT INTO workspace_app.projects(
			id, workspace_id, name, display_name, namespace, language, git_url,
			git_ref_type, git_ref_value, dockerfile_path, template_id, secret_id,
			review_status, resources, default_cluster_id, jenkins_secret_id, kubernetes_secret_id)
		 VALUES (?, ?, 'orders', 'Orders', 'team', 'Go', 'https://git.test.invalid/team/orders',
		         'branch', 'main', 'Dockerfile', '', ?, 'approved', ?, ?, ?, ?)`,
		projectID.String(), wsID.String(), secretIDs["GitLab"].String(), rawResources,
		clusterID.String(), secretIDs["Jenkins"].String(), secretIDs["Kubernetes"].String(),
	).Error)
}

func (f *pipelineFixture) create(t *testing.T, input command.CreateInput) *model.DeploymentRun {
	t.Helper()
	run, err := f.createHandler.Handle(f.ctx, f.wsID, f.memberID, f.projectID, input)
	require.NoError(t, err)
	return run
}

func (f *pipelineFixture) createAndObserve(t *testing.T, input command.CreateInput) (*model.DeploymentRun, []model.Status) {
	t.Helper()
	run := f.create(t, input)
	statuses := f.observe(t, run)
	return run, statuses
}

func (f *pipelineFixture) observe(t *testing.T, run *model.DeploymentRun) []model.Status {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, 8*time.Second)
	defer cancel()
	ch, initial, done, unsubscribe, err := f.stream.Subscribe(ctx, f.wsID, run.ID)
	require.NoError(t, err)
	defer unsubscribe()
	statuses := decodeStatuses(t, initial)
	if done {
		return statuses
	}
	for {
		select {
		case delta, open := <-ch:
			if !open {
				return statuses
			}
			statuses = append(statuses, decodeStatuses(t, delta)...)
		case <-ctx.Done():
			t.Fatalf("deployment stream timed out with statuses %v", statuses)
		}
	}
}

func (f *pipelineFixture) requireCreateRejected(t *testing.T) {
	t.Helper()
	_, err := f.createHandler.Handle(f.ctx, f.wsID, f.memberID, f.projectID,
		command.CreateInput{GitRefType: "branch", GitRefValue: "main"})
	require.ErrorIs(t, err, domain.ErrDeploymentBindingMissing)
	f.requireNoRunsOrMessages(t)
}

func (f *pipelineFixture) requireNoRunsOrMessages(t *testing.T) {
	t.Helper()
	page, pageErr := f.repo.ListPageByProject(f.ctx, f.wsID, f.projectID, 1, 20)
	require.NoError(t, pageErr)
	require.Zero(t, page.Total)
	require.Zero(t, f.queue.publishCount())
}

func (f *pipelineFixture) requireActiveConflict(t *testing.T) {
	t.Helper()
	first := f.create(t, command.CreateInput{GitRefType: "branch", GitRefValue: "main"})
	_, err := f.createHandler.Handle(f.ctx, f.wsID, f.memberID, f.projectID,
		command.CreateInput{GitRefType: "branch", GitRefValue: "main"})
	require.ErrorIs(t, err, domain.ErrDeploymentAlreadyRunning)
	f.jenkins.unblock()
	require.Equal(t, model.StatusSucceeded, f.observe(t, first)[4])
}

func (f *pipelineFixture) requireReconcilerRecovery(t *testing.T) {
	t.Helper()
	run := f.create(t, command.CreateInput{GitRefType: "branch", GitRefValue: "main"})
	select {
	case <-f.builds.terminal:
	case <-time.After(8 * time.Second):
		t.Fatal("mock Jenkins build did not complete")
	}
	before, err := f.repo.FindByID(f.ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, model.StatusBuilding, before.Status)
	require.NoError(t, f.reconciler.RunOnce(f.ctx))
	statuses := f.observe(t, run)
	require.Equal(t, model.StatusSucceeded, statuses[len(statuses)-1])
}

func (f *pipelineFixture) requireAbortCompletesStream(t *testing.T) {
	t.Helper()
	run := f.create(t, command.CreateInput{GitRefType: "branch", GitRefValue: "main"})
	ch, initial, done, unsubscribe, err := f.stream.Subscribe(f.ctx, f.wsID, run.ID)
	require.NoError(t, err)
	defer unsubscribe()
	require.False(t, done)
	_, err = f.abort.Handle(f.ctx, f.wsID, run.ID)
	require.NoError(t, err)
	statuses := decodeStatuses(t, initial)
	for delta := range ch {
		statuses = append(statuses, decodeStatuses(t, delta)...)
	}
	require.Equal(t, model.StatusAborted, statuses[len(statuses)-1])
	f.jenkins.unblock()
}

func decodeStatuses(t *testing.T, output string) []model.Status {
	t.Helper()
	var statuses []model.Status
	for _, line := range splitNonEmptyLines(output) {
		var payload struct {
			Status model.Status `json:"status"`
		}
		require.NoError(t, json.Unmarshal([]byte(line), &payload))
		statuses = append(statuses, payload.Status)
	}
	return statuses
}

func splitNonEmptyLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func jsonKind(raw string) string {
	if strings.Contains(raw, "\nkind: Deployment\n") || strings.HasPrefix(raw, "kind: Deployment\n") {
		return "Deployment"
	}
	return ""
}

func TestDeploymentPipelineAcceptance(t *testing.T) {
	env := newPipelineFixture(t, pipelineModeSuccess)
	run, statuses := env.createAndObserve(t, command.CreateInput{
		GitRefType: "branch", GitRefValue: "main",
	})

	require.Equal(t, "main", run.ImageTag)
	require.Equal(t, []model.Status{
		model.StatusQueued, model.StatusPreparing, model.StatusBuilding,
		model.StatusDeploying, model.StatusSucceeded,
	}, statuses)
	require.Contains(t, env.runtime.lastDeploymentYAML, "registry.test.invalid/team/orders:"+run.ImageTag)
	require.NotEmpty(t, env.runtime.lastKubeconfig)
	require.Equal(t, "registry.test.invalid/team/orders", env.jenkins.LastBuildRequest.ImageRepository)
	require.Equal(t, run.ImageTag, env.jenkins.LastBuildRequest.ImageTag)
	require.Equal(t, 2, env.runtime.applyCalls, "successful pipeline applies both deployment stages")
}

func TestDeploymentPipelineCIFailureHasZeroApply(t *testing.T) {
	env := newPipelineFixture(t, pipelineModeCIFailure)
	_, statuses := env.createAndObserve(t, command.CreateInput{GitRefType: "branch", GitRefValue: "main"})

	require.Equal(t, model.StatusFailed, statuses[len(statuses)-1])
	require.Equal(t, 1, env.runtime.applyCalls)
}

func TestDeploymentPipelineMissingHarborFailsCreate(t *testing.T) {
	env := newPipelineFixture(t, pipelineModeMissingHarbor)
	env.requireCreateRejected(t)
}

func TestUnauthorizedClusterRejectedAtCreate(t *testing.T) {
	env := newPipelineFixture(t, pipelineModeSuccess)

	_, err := env.createHandler.Handle(env.ctx, env.wsID, env.memberID, env.projectID,
		command.CreateInput{
			ClusterID:  env.unauthorizedClusterID.String(),
			GitRefType: "branch", GitRefValue: "main",
		})

	require.ErrorIs(t, err, domain.ErrDeploymentClusterNotAuthorized)
	env.requireNoRunsOrMessages(t)
}

func TestDeploymentPipelineActiveConflict(t *testing.T) {
	env := newPipelineFixture(t, pipelineModeBlockedBuild)
	env.requireActiveConflict(t)
}

func TestDeploymentPipelineReconcilerRecoversMissedBuildCompletion(t *testing.T) {
	env := newPipelineFixture(t, pipelineModeReconcile)
	env.requireReconcilerRecovery(t)
}

func TestDeploymentPipelineAbortCompletesStream(t *testing.T) {
	env := newPipelineFixture(t, pipelineModeBlockedBuild)
	env.requireAbortCompletesStream(t)
}

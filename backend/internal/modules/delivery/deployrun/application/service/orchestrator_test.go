package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	deploycommand "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deployport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/runner"
	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type orchestratorRepo struct {
	run         *model.DeploymentRun
	findErr     error
	updateErr   error
	updates     int
	active      []*model.DeploymentRun
	afterUpdate *model.DeploymentRun
	mu          sync.Mutex
	leased      bool
	listEntered chan struct{}
	listRelease chan struct{}
	events      []string
}

func (r *orchestratorRepo) Create(context.Context, *model.DeploymentRun) error { return nil }
func (r *orchestratorRepo) FindByID(context.Context, kernel.DeploymentRunID) (*model.DeploymentRun, error) {
	return r.run, r.findErr
}
func (r *orchestratorRepo) FindByBuildID(context.Context, kernel.BuildID) (*model.DeploymentRun, error) {
	return r.run, r.findErr
}
func (r *orchestratorRepo) Update(_ context.Context, run *model.DeploymentRun) error {
	r.updates++
	if r.afterUpdate != nil {
		r.run = r.afterUpdate
	}
	if r.updateErr == nil {
		run.Version++
		if run.LeaseOwner == "" {
			r.leased = false
		}
	}
	return r.updateErr
}
func (r *orchestratorRepo) ListByProject(context.Context, kernel.ProjectID, int) ([]*model.DeploymentRun, error) {
	return nil, nil
}
func (r *orchestratorRepo) ListActive(context.Context) ([]*model.DeploymentRun, error) {
	if r.listEntered != nil {
		close(r.listEntered)
		<-r.listRelease
	}
	if r.active != nil {
		return r.active, r.findErr
	}
	if r.run.AttemptCount >= 3 || r.run.Status == model.StatusFailed ||
		r.run.Status == model.StatusAborted || r.run.Status == model.StatusSucceeded ||
		(r.run.LeaseOwner != "" && r.run.LeaseExpiresAt != nil && r.run.LeaseExpiresAt.After(time.Now())) {
		return nil, r.findErr
	}
	return []*model.DeploymentRun{r.run}, r.findErr
}
func (r *orchestratorRepo) ListEvents(context.Context, kernel.DeploymentRunID) ([]model.DeploymentRunEvent, error) {
	return nil, nil
}
func (r *orchestratorRepo) AppendEvent(_ context.Context, _ *model.DeploymentRun, message string) error {
	r.events = append(r.events, message)
	return nil
}
func (r *orchestratorRepo) HasActiveByProject(context.Context, kernel.ProjectID) (bool, error) {
	return false, nil
}
func (r *orchestratorRepo) TryAcquireLease(_ context.Context, _ kernel.DeploymentRunID, expected int64, owner string, ttl time.Duration) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.leased || r.run.Version != expected {
		return false, nil
	}
	r.leased = true
	return true, nil
}
func (r *orchestratorRepo) ReleaseLease(context.Context, kernel.DeploymentRunID, int64, string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.leased = false
	r.run.LeaseOwner = ""
	r.run.LeaseExpiresAt = nil
	r.run.Version++
	return nil
}
func (r *orchestratorRepo) RecordInfrastructureFailure(_ context.Context, runID kernel.DeploymentRunID, expected int64, message string) (*model.DeploymentRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run := r.run
	if run == nil {
		for _, candidate := range r.active {
			if candidate.ID == runID {
				run = candidate
				break
			}
		}
	}
	if run == nil || run.Version != expected {
		return nil, domain.ErrDeploymentRunConcurrentUpdate
	}
	run.AttemptCount++
	run.LeaseOwner = ""
	run.LeaseExpiresAt = nil
	r.leased = false
	if run.AttemptCount >= 3 {
		run.Fail(message, time.Now())
		run.AttemptCount = 3
	}
	run.Version++
	return run, nil
}

type orchestratorProjects struct {
	project  deployport.ProjectDeploymentContext
	clusters []deployport.DeploymentCluster
	err      error
	loads    int
	loadIDs  []kernel.ClusterInfoID
}

func (p *orchestratorProjects) Load(_ context.Context, _ kernel.WorkspaceID, _ kernel.ProjectID, clusterID kernel.ClusterInfoID) (deployport.ProjectDeploymentContext, error) {
	p.loads++
	p.loadIDs = append(p.loadIDs, clusterID)
	if p.err != nil {
		return deployport.ProjectDeploymentContext{}, p.err
	}
	out := p.project
	for _, cluster := range p.clusters {
		if cluster.ID == clusterID {
			out.Cluster = cluster
			return out, nil
		}
	}
	return out, nil
}
func (p *orchestratorProjects) ListAuthorizedClusters(context.Context, kernel.WorkspaceID) ([]deployport.DeploymentCluster, error) {
	if len(p.clusters) > 0 {
		return p.clusters, nil
	}
	return []deployport.DeploymentCluster{p.project.Cluster}, nil
}

type orchestratorPrepare struct {
	calls int
	err   error
}

func (p *orchestratorPrepare) Run(context.Context, deployport.ProjectDeploymentContext, PrepareInput) error {
	p.calls++
	return p.err
}

type orchestratorTrigger struct {
	calls   int
	req     deliveryport.BuildRequest
	err     error
	build   *buildmodel.Build
	started bool
}

func (t *orchestratorTrigger) TriggerDeploymentAssociated(ctx context.Context, _ kernel.WorkspaceID, _ kernel.MemberID, _ kernel.ProjectID, req deliveryport.BuildRequest, beforeStart func(context.Context, *buildmodel.Build) error) (*buildmodel.Build, error) {
	t.calls++
	t.req = req
	if t.err == nil && beforeStart != nil {
		if err := beforeStart(ctx, t.build); err != nil {
			return nil, err
		}
	}
	t.started = true
	return t.build, t.err
}

type orchestratorRenderer struct {
	calls int
	tag   string
}

func (r *orchestratorRenderer) Render(_ context.Context, project deployport.ProjectDeploymentContext, tag string) ([]runtimeport.Manifest, error) {
	r.calls++
	r.tag = tag
	return []runtimeport.Manifest{
		{YAML: `apiVersion: v1
kind: Service
metadata:
  name: ` + project.Name + `
  namespace: ` + project.Namespace},
		{YAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ` + project.Name + `
  namespace: ` + project.Namespace + `
spec:
  template:
    spec:
      containers:
        - name: app
          image: ` + project.ImageRepository + `:` + tag},
	}, nil
}

type orchestratorDeployer struct {
	calls        int
	restartCalls int
	target       runtimeport.DeploymentTarget
	manifests    []runtimeport.Manifest
	err          error
	errAfter     int
	restartErr   error
}

func (d *orchestratorDeployer) Deploy(_ context.Context, target runtimeport.DeploymentTarget, manifests []runtimeport.Manifest) error {
	d.calls++
	d.target = target
	d.manifests = manifests
	if d.err != nil && (d.errAfter == 0 || d.calls > d.errAfter) {
		return d.err
	}
	return nil
}

func (d *orchestratorDeployer) RolloutRestart(_ context.Context, target runtimeport.DeploymentTarget) error {
	d.restartCalls++
	d.target = target
	return d.restartErr
}

type orchestratorPublisher struct {
	calls int
	err   error
	errs  []error
}

type orchestratorBuilds struct {
	build *buildmodel.Build
	err   error
}

func (b *orchestratorBuilds) FindByID(context.Context, kernel.BuildID) (*buildmodel.Build, error) {
	return b.build, b.err
}

func (p *orchestratorPublisher) PublishRun(context.Context, *model.DeploymentRun) error {
	p.calls++
	if len(p.errs) >= p.calls {
		return p.errs[p.calls-1]
	}
	return p.err
}

type orchestratorFixture struct {
	run       *model.DeploymentRun
	wsID      kernel.WorkspaceID
	repo      *orchestratorRepo
	projects  *orchestratorProjects
	prepare   *orchestratorPrepare
	trigger   *orchestratorTrigger
	renderer  *orchestratorRenderer
	deployer  *orchestratorDeployer
	publisher *orchestratorPublisher
	builds    *orchestratorBuilds
	o         *Orchestrator
}

func newOrchestratorFixture(status model.Status) *orchestratorFixture {
	wsID := kernel.NewWorkspaceID()
	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "run-tag")
	run.Status = status
	project := deployport.ProjectDeploymentContext{
		WorkspaceID: wsID, ProjectID: run.ProjectID, Name: "orders", Namespace: "team-a",
		ImageRepository: "registry.example/team-a/orders",
		Cluster:         deployport.DeploymentCluster{ID: run.ClusterID, Server: "https://cluster.example"},
		Kubernetes:      deployport.DeploymentSecret{Credentials: deployport.DeploymentCredentials{ExtraConfig: "same-source-kubeconfig"}},
	}
	f := &orchestratorFixture{
		run: run, wsID: wsID, repo: &orchestratorRepo{run: run},
		projects: &orchestratorProjects{project: project}, prepare: &orchestratorPrepare{},
		trigger: &orchestratorTrigger{}, renderer: &orchestratorRenderer{},
		deployer: &orchestratorDeployer{}, publisher: &orchestratorPublisher{},
	}
	f.trigger.build = buildmodel.NewBuild(run.ProjectID, run.CreatedBy, wsID, run.GitRefValue)
	f.builds = &orchestratorBuilds{build: f.trigger.build}
	f.o = NewOrchestrator(f.repo, f.builds, f.projects, f.prepare, f.trigger, f.renderer, f.deployer, f.publisher, 10*time.Minute, 30*time.Minute)
	return f
}

func TestQueuedMessageOnlyAdvancesToPreparingAndRepublishes(t *testing.T) {
	f := newOrchestratorFixture(model.StatusQueued)
	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))
	require.Equal(t, model.StatusPreparing, f.run.Status)
	require.Equal(t, 1, f.repo.updates)
	require.Equal(t, 1, f.publisher.calls)
	require.Zero(t, f.prepare.calls)
}

func TestPreparingUpsertsAndTriggersExactlyOnce(t *testing.T) {
	f := newOrchestratorFixture(model.StatusPreparing)
	f.run.AttemptCount = 2
	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))
	require.Equal(t, model.StatusBuilding, f.run.Status)
	require.Zero(t, f.run.AttemptCount)
	require.Equal(t, 1, f.prepare.calls)
	require.Equal(t, 1, f.trigger.calls)
	require.Equal(t, 1, f.deployer.calls)
	require.True(t, f.deployer.target.SkipRolloutWait)
	require.Equal(t, f.run.ImageTag, f.trigger.req.ImageTag)
	require.Equal(t, f.projects.project.ImageRepository, f.trigger.req.ImageRepository)
	require.Equal(t, 30*time.Minute, f.trigger.req.Timeout)
	require.Equal(t, f.trigger.build.ID, *f.run.BuildID)
}

func TestBuildingMessageDoesNotInvokeJenkinsAgain(t *testing.T) {
	f := newOrchestratorFixture(model.StatusBuilding)
	buildID := f.trigger.build.ID
	f.run.BuildID = &buildID
	started := time.Now()
	f.builds.build.ExecutionStartedAt = &started
	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))
	require.Zero(t, f.prepare.calls)
	require.Zero(t, f.trigger.calls)
	require.Zero(t, f.deployer.calls)
}

func TestBuildingWithoutExecutionStartFailsInsteadOfWaitingForever(t *testing.T) {
	f := newOrchestratorFixture(model.StatusBuilding)
	buildID := f.trigger.build.ID
	f.run.BuildID = &buildID
	f.builds.build.ExecutionStartedAt = nil

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusFailed, f.run.Status)
}

func TestBuildingPastJenkinsTimeoutFails(t *testing.T) {
	f := newOrchestratorFixture(model.StatusBuilding)
	buildID := f.trigger.build.ID
	f.run.BuildID = &buildID
	started := time.Now().Add(-31 * time.Minute)
	f.builds.build.ExecutionStartedAt = &started

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusFailed, f.run.Status)
}

func TestBuildingMessageRecoversSuccessfulBuildAndPublishesDeploy(t *testing.T) {
	f := newOrchestratorFixture(model.StatusBuilding)
	buildID := f.trigger.build.ID
	f.run.BuildID = &buildID
	f.builds.build.Succeed(1, "team-a/orders", "ok")

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusDeploying, f.run.Status)
	require.Equal(t, 1, f.publisher.calls)
	require.Zero(t, f.deployer.calls)
}

func TestBuildingMessageRecoversFailedBuildWithoutApply(t *testing.T) {
	f := newOrchestratorFixture(model.StatusBuilding)
	buildID := f.trigger.build.ID
	f.run.BuildID = &buildID
	f.builds.build.Fail("ci failed", "log")

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusFailed, f.run.Status)
	require.Zero(t, f.publisher.calls)
	require.Zero(t, f.deployer.calls)
}

func TestBuildingRecoveryDoesNotOverwriteConcurrentAbort(t *testing.T) {
	f := newOrchestratorFixture(model.StatusBuilding)
	buildID := f.trigger.build.ID
	f.run.BuildID = &buildID
	f.builds.build.Succeed(1, "team-a/orders", "ok")
	latest := *f.run
	latest.Status = model.StatusAborted
	f.repo.afterUpdate = &latest
	f.repo.updateErr = domain.ErrDeploymentRunConcurrentUpdate

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusAborted, f.repo.run.Status)
	require.Zero(t, f.publisher.calls)
}

func TestDeployingUsesSingleAuthorizedContextAndRunTag(t *testing.T) {
	f := newOrchestratorFixture(model.StatusDeploying)
	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))
	require.Equal(t, model.StatusSucceeded, f.run.Status)
	require.Equal(t, 1, f.deployer.calls)
	require.Equal(t, 1, f.deployer.restartCalls)
	require.True(t, f.deployer.target.SkipRolloutWait)
	require.Equal(t, f.projects.project.Cluster.ID.String(), f.deployer.target.ClusterID)
	require.Equal(t, f.projects.project.Cluster.Server, f.deployer.target.Server)
	require.Equal(t, f.projects.project.Kubernetes.Credentials.ExtraConfig, f.deployer.target.Kubeconfig)
	require.Equal(t, f.run.ImageTag, f.deployer.target.ImageTag)
	require.Equal(t, f.run.ImageTag, f.renderer.tag)
}

func TestBusinessFailureIsPersistedAndAcknowledged(t *testing.T) {
	f := newOrchestratorFixture(model.StatusQueued)
	f.projects.err = domain.ErrDeploymentBindingMissing
	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))
	require.Equal(t, model.StatusFailed, f.run.Status)
	require.Equal(t, 1, f.repo.updates)
}

func TestRepositoryFailureIsReturnedForRetry(t *testing.T) {
	f := newOrchestratorFixture(model.StatusQueued)
	f.repo.findErr = errors.New("db unavailable")
	require.Error(t, f.o.Step(context.Background(), f.run.ID, f.wsID))
	require.Zero(t, f.repo.updates)
}

func TestTriggerInfrastructureFailureIsReturnedForRetry(t *testing.T) {
	f := newOrchestratorFixture(model.StatusPreparing)
	f.trigger.err = errors.New("build database unavailable")
	require.Error(t, f.o.Step(context.Background(), f.run.ID, f.wsID))
	require.Equal(t, model.StatusPreparing, f.run.Status)
	require.Zero(t, f.repo.updates)
	require.Equal(t, 1, f.run.AttemptCount)
}

func TestInfrastructureFailureExhaustsGlobalAttemptBudget(t *testing.T) {
	f := newOrchestratorFixture(model.StatusPreparing)
	f.trigger.err = errors.New("jenkins unavailable")

	require.Error(t, f.o.Step(context.Background(), f.run.ID, f.wsID))
	require.Error(t, f.o.Step(context.Background(), f.run.ID, f.wsID))
	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusFailed, f.run.Status)
	require.Equal(t, 3, f.run.AttemptCount)
	require.False(t, f.repo.leased)
	require.NoError(t, NewReconciler(f.repo, f.publisher).RunOnce(context.Background()))
	require.Zero(t, f.publisher.calls)
}

type blockingDeployer struct {
	entered chan struct{}
	release chan struct{}
	calls   int
	mu      sync.Mutex
}

func (d *blockingDeployer) Deploy(context.Context, runtimeport.DeploymentTarget, []runtimeport.Manifest) error {
	d.mu.Lock()
	d.calls++
	call := d.calls
	d.mu.Unlock()
	if call == 1 {
		close(d.entered)
		<-d.release
	}
	return nil
}

func (d *blockingDeployer) RolloutRestart(context.Context, runtimeport.DeploymentTarget) error {
	return nil
}

func TestConcurrentOrchestratorsLeaseAllowsOnlyOneApply(t *testing.T) {
	f := newOrchestratorFixture(model.StatusDeploying)
	blocker := &blockingDeployer{entered: make(chan struct{}), release: make(chan struct{})}
	first := NewOrchestrator(f.repo, f.builds, f.projects, f.prepare, f.trigger, f.renderer, blocker, f.publisher, time.Minute, time.Minute)
	second := NewOrchestrator(f.repo, f.builds, f.projects, f.prepare, f.trigger, f.renderer, blocker, f.publisher, time.Minute, time.Minute)
	done := make(chan error, 1)
	go func() { done <- first.Step(context.Background(), f.run.ID, f.wsID) }()
	<-blocker.entered

	require.NoError(t, second.Step(context.Background(), f.run.ID, f.wsID))
	close(blocker.release)
	require.NoError(t, <-done)
	require.Equal(t, 1, blocker.calls)
}

type contextBlockingPreparer struct {
	entered chan struct{}
	calls   int
	mu      sync.Mutex
}

func (p *contextBlockingPreparer) Run(ctx context.Context, _ deployport.ProjectDeploymentContext, _ PrepareInput) error {
	p.mu.Lock()
	p.calls++
	call := p.calls
	p.mu.Unlock()
	if call == 1 {
		close(p.entered)
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestPreparingOperationContextCancelsBeforeLeaseGraceAndBlocksSecondOwner(t *testing.T) {
	f := newOrchestratorFixture(model.StatusPreparing)
	blocker := &contextBlockingPreparer{entered: make(chan struct{})}
	first := NewOrchestrator(f.repo, f.builds, f.projects, blocker, f.trigger, f.renderer, f.deployer, f.publisher, time.Minute, 20*time.Millisecond)
	second := NewOrchestrator(f.repo, f.builds, f.projects, blocker, f.trigger, f.renderer, f.deployer, f.publisher, time.Minute, 20*time.Millisecond)
	done := make(chan error, 1)
	started := time.Now()
	go func() { done <- first.Step(context.Background(), f.run.ID, f.wsID) }()
	<-blocker.entered

	require.NoError(t, second.Step(context.Background(), f.run.ID, f.wsID))
	require.ErrorIs(t, <-done, context.DeadlineExceeded)
	require.Less(t, time.Since(started), 100*time.Millisecond)
	require.Equal(t, 1, blocker.calls)
}

type contextBlockingDeployer struct {
	entered chan struct{}
	calls   int
	mu      sync.Mutex
}

func (d *contextBlockingDeployer) Deploy(ctx context.Context, _ runtimeport.DeploymentTarget, _ []runtimeport.Manifest) error {
	d.mu.Lock()
	d.calls++
	call := d.calls
	d.mu.Unlock()
	if call == 1 {
		close(d.entered)
	}
	<-ctx.Done()
	return ctx.Err()
}

func (d *contextBlockingDeployer) RolloutRestart(context.Context, runtimeport.DeploymentTarget) error {
	return nil
}

func TestDeployingOperationContextCoversApplyAndBlocksSecondOwner(t *testing.T) {
	f := newOrchestratorFixture(model.StatusDeploying)
	blocker := &contextBlockingDeployer{entered: make(chan struct{})}
	first := NewOrchestrator(f.repo, f.builds, f.projects, f.prepare, f.trigger, f.renderer, blocker, f.publisher, 20*time.Millisecond, time.Minute)
	second := NewOrchestrator(f.repo, f.builds, f.projects, f.prepare, f.trigger, f.renderer, blocker, f.publisher, 20*time.Millisecond, time.Minute)
	done := make(chan error, 1)
	go func() { done <- first.Step(context.Background(), f.run.ID, f.wsID) }()
	<-blocker.entered

	require.NoError(t, second.Step(context.Background(), f.run.ID, f.wsID))
	require.ErrorIs(t, <-done, context.DeadlineExceeded)
	require.Equal(t, 1, blocker.calls)
}

func TestAbortDeployingCancelsDeployerAndPreventsSuccess(t *testing.T) {
	f := newOrchestratorFixture(model.StatusDeploying)
	blocker := &contextBlockingDeployer{entered: make(chan struct{})}
	operations := runner.NewRegistry()
	orchestrator := NewOrchestrator(
		f.repo, f.builds, f.projects, f.prepare, f.trigger, f.renderer, blocker, f.publisher,
		time.Minute, time.Minute, operations,
	)
	abort := deploycommand.NewAbortDeploymentHandler(
		f.repo, time.Now, deploycommand.AbortDependencies{Runs: operations},
	)
	stepDone := make(chan error, 1)
	go func() { stepDone <- orchestrator.Step(context.Background(), f.run.ID, f.wsID) }()
	<-blocker.entered

	aborted, err := abort.Handle(context.Background(), f.wsID, f.run.ID)

	require.NoError(t, err)
	require.Equal(t, model.StatusAborted, aborted.Status)
	require.ErrorIs(t, <-stepDone, context.Canceled)
	require.Equal(t, model.StatusAborted, f.run.Status)
	require.Equal(t, 1, blocker.calls)
}

func TestPreparingAssociationConcurrentAbortDoesNotStartBuild(t *testing.T) {
	f := newOrchestratorFixture(model.StatusPreparing)
	latest := *f.run
	latest.Status = model.StatusAborted
	f.repo.afterUpdate = &latest
	f.repo.updateErr = domain.ErrDeploymentRunConcurrentUpdate

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.False(t, f.trigger.started)
	require.Equal(t, model.StatusAborted, f.repo.run.Status)
}

func TestWrongWorkspaceMessageIsIgnored(t *testing.T) {
	f := newOrchestratorFixture(model.StatusQueued)
	require.NoError(t, f.o.Step(context.Background(), f.run.ID, kernel.NewWorkspaceID()))
	require.Zero(t, f.projects.loads)
	require.Zero(t, f.repo.updates)
}

func TestOrchestratorMockClosureAdvancesOneStagePerDelivery(t *testing.T) {
	f := newOrchestratorFixture(model.StatusQueued)
	ctx := context.Background()

	require.NoError(t, f.o.Step(ctx, f.run.ID, f.wsID))
	require.Equal(t, model.StatusPreparing, f.run.Status)
	require.Zero(t, f.trigger.calls)

	require.NoError(t, f.o.Step(ctx, f.run.ID, f.wsID))
	require.Equal(t, model.StatusBuilding, f.run.Status)
	require.Equal(t, 1, f.trigger.calls)
	require.Equal(t, 1, f.deployer.calls)

	require.NoError(t, f.run.Transition(model.StatusDeploying, time.Now()))
	require.NoError(t, f.o.Step(ctx, f.run.ID, f.wsID))
	require.Equal(t, model.StatusSucceeded, f.run.Status)
	require.Equal(t, 1, f.trigger.calls)
	require.Equal(t, 2, f.deployer.calls)
	require.Equal(t, 1, f.deployer.restartCalls)

	require.NoError(t, f.o.Step(ctx, f.run.ID, f.wsID))
	require.Equal(t, 1, f.trigger.calls)
	require.Equal(t, 2, f.deployer.calls)
}

func TestDeployingCrossClusterSyncsOtherClusters(t *testing.T) {
	f := newOrchestratorFixture(model.StatusDeploying)
	f.run.CrossCluster = true
	f.run.ImageTag = "sync-tag"
	syncCluster := deployport.DeploymentCluster{
		ID: kernel.NewClusterInfoID(), Name: "sync-env", Server: "https://sync.example",
	}
	f.projects.clusters = []deployport.DeploymentCluster{
		{ID: f.run.ClusterID, Name: "build-env", Server: "https://cluster.example", IsBuildEnv: true},
		syncCluster,
	}

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusSucceeded, f.run.Status)
	require.Equal(t, 2, f.deployer.calls)
	require.Equal(t, 2, f.deployer.restartCalls)
	require.Contains(t, f.repo.events, "cross-cluster sync started: sync-env")
	require.Contains(t, f.repo.events, "cross-cluster sync succeeded: sync-env")
	require.Equal(t, syncCluster.ID.String(), f.deployer.target.ClusterID)
}

func TestDeployingCrossClusterFailsWhenSyncFails(t *testing.T) {
	f := newOrchestratorFixture(model.StatusDeploying)
	f.run.CrossCluster = true
	syncCluster := deployport.DeploymentCluster{
		ID: kernel.NewClusterInfoID(), Name: "sync-env", Server: "https://sync.example",
	}
	f.projects.clusters = []deployport.DeploymentCluster{
		{ID: f.run.ClusterID, Name: "build-env", Server: "https://cluster.example", IsBuildEnv: true},
		syncCluster,
	}
	f.deployer.errAfter = 1
	f.deployer.err = errors.New("apply failed")

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusFailed, f.run.Status)
	require.Contains(t, f.run.ErrorMessage, "sync-env")
	require.Contains(t, f.repo.events[len(f.repo.events)-1], "cross-cluster sync failed: sync-env")
}

func TestDeployingSyncOnlyAppliesUserManifestsToTargetsOnly(t *testing.T) {
	f := newOrchestratorFixture(model.StatusDeploying)
	f.run.CrossCluster = true
	f.run.RunType = model.RunTypeSync
	f.run.ImageTag = "sync-tag"
	f.run.SyncManifests = []model.SyncManifest{
		{
			Kind: model.SyncManifestKindConfigMap,
			Name: "app-config",
			YAML: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app-config\ndata:\n  k: v\n",
		},
		{Kind: model.SyncManifestKindSecret, Name: "registry-pull", ManagedPullSecret: true},
	}
	syncCluster := deployport.DeploymentCluster{
		ID: kernel.NewClusterInfoID(), Name: "sync-env", Server: "https://sync.example",
	}
	f.projects.clusters = []deployport.DeploymentCluster{
		{ID: f.run.ClusterID, Name: "build-env", Server: "https://cluster.example", IsBuildEnv: true},
		syncCluster,
	}

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusSucceeded, f.run.Status)
	require.Equal(t, 1, f.deployer.calls)
	require.Equal(t, 0, f.deployer.restartCalls)
	require.True(t, f.deployer.target.AllowPartialWorkloads)
	require.Equal(t, []string{"registry-pull"}, f.deployer.target.ExtraImagePullSecrets)
	require.Len(t, f.deployer.manifests, 1)
	require.Contains(t, f.deployer.manifests[0].YAML, "ConfigMap")
	require.Equal(t, syncCluster.ID.String(), f.deployer.target.ClusterID)
	require.Contains(t, f.repo.events, "applied: ConfigMap/app-config, Secret/registry-pull")
	require.Contains(t, f.repo.events, "cross-cluster sync succeeded: sync-env")
}

func TestDeployingSyncOnlyWithDeploymentRestarts(t *testing.T) {
	f := newOrchestratorFixture(model.StatusDeploying)
	f.run.CrossCluster = true
	f.run.RunType = model.RunTypeSync
	f.run.ImageTag = "sync-tag"
	f.run.SyncManifests = []model.SyncManifest{{
		Kind: model.SyncManifestKindDeployment,
		Name: "demo",
		YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: demo\nspec:\n  template:\n    spec:\n      containers:\n        - name: app\n          image: repo/app:sync-tag\n",
	}}
	syncCluster := deployport.DeploymentCluster{
		ID: kernel.NewClusterInfoID(), Name: "sync-env", Server: "https://sync.example",
	}
	f.projects.clusters = []deployport.DeploymentCluster{
		{ID: f.run.ClusterID, Name: "build-env", Server: "https://cluster.example", IsBuildEnv: true},
		syncCluster,
	}

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusSucceeded, f.run.Status)
	require.Equal(t, 1, f.deployer.calls)
	require.Equal(t, 1, f.deployer.restartCalls)
	require.Equal(t, "demo", f.deployer.target.DeploymentName)
}

func TestEnvironmentSyncRunAppliesToTargetWithoutBuildOrCrossCluster(t *testing.T) {
	f := newOrchestratorFixture(model.StatusDeploying)
	f.run = model.NewEnvironmentSyncDeploymentRun(f.wsID, f.run.ProjectID, f.run.ClusterID, f.run.CreatedBy)
	f.run.Status = model.StatusDeploying
	f.run.DeployNamespace = "target-ns"
	f.repo.run = f.run
	f.run.SyncManifests = []model.SyncManifest{
		{Kind: model.SyncManifestKindSecret, Name: "registry-pull", ManagedPullSecret: true},
		{
			Kind: model.SyncManifestKindConfigMap,
			Name: "app-config",
			YAML: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app-config\n  namespace: target-ns\ndata:\n  k: v\n",
		},
		{
			Kind: model.SyncManifestKindDeployment,
			Name: "demo",
			YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: demo\n  namespace: target-ns\nspec:\n  template:\n    spec:\n      containers:\n        - name: app\n          image: repo/app:v2\n",
		},
	}
	f.projects.project.Namespace = "target-ns"

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusSucceeded, f.run.Status)
	require.Zero(t, f.prepare.calls)
	require.Zero(t, f.trigger.calls)
	require.Equal(t, 3, f.deployer.calls)
	require.Equal(t, 1, f.deployer.restartCalls)
	require.Equal(t, "target-ns", f.deployer.target.Namespace)
	require.Contains(t, f.repo.events, "applied: Secret/registry-pull")
	require.Contains(t, f.repo.events, "applied: ConfigMap/app-config")
	require.Contains(t, f.repo.events, "applied: Deployment/demo")
}

func TestEnvironmentSyncRunReportsPartialApplyFailure(t *testing.T) {
	f := newOrchestratorFixture(model.StatusDeploying)
	f.run = model.NewEnvironmentSyncDeploymentRun(f.wsID, f.run.ProjectID, f.run.ClusterID, f.run.CreatedBy)
	f.run.Status = model.StatusDeploying
	f.run.DeployNamespace = "target-ns"
	f.repo.run = f.run
	f.run.SyncManifests = []model.SyncManifest{
		{
			Kind: model.SyncManifestKindConfigMap,
			Name: "app-config",
			YAML: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app-config\n  namespace: target-ns\ndata:\n  k: v\n",
		},
		{
			Kind: model.SyncManifestKindDeployment,
			Name: "demo",
			YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: demo\n  namespace: target-ns\n",
		},
	}
	f.deployer.errAfter = 1
	f.deployer.err = errors.New("apply failed")

	require.NoError(t, f.o.Step(context.Background(), f.run.ID, f.wsID))

	require.Equal(t, model.StatusFailed, f.run.Status)
	require.Equal(t, 2, f.deployer.calls)
	require.Contains(t, f.repo.events, "applied: ConfigMap/app-config")
	require.Contains(t, f.repo.events[len(f.repo.events)-1], "failed: Deployment/demo")
}

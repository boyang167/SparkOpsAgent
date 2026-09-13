package subscriber

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type subscriberRepo struct {
	run         *model.DeploymentRun
	findErr     error
	updateErr   error
	updates     int
	afterUpdate *model.DeploymentRun
	cas         bool
	findBarrier chan struct{}
	findCalls   int
	mu          sync.Mutex
}

func (r *subscriberRepo) Create(context.Context, *model.DeploymentRun) error { return nil }
func (r *subscriberRepo) Update(_ context.Context, run *model.DeploymentRun) error {
	return r.update(run)
}
func (r *subscriberRepo) update(run *model.DeploymentRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates++
	if r.cas {
		if run.Version != r.run.Version {
			return domain.ErrDeploymentRunConcurrentUpdate
		}
		copy := *run
		copy.Version++
		r.run = &copy
		return nil
	}
	if r.afterUpdate != nil {
		r.run = r.afterUpdate
	}
	return r.updateErr
}
func (r *subscriberRepo) FindByID(context.Context, kernel.DeploymentRunID) (*model.DeploymentRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.run, r.findErr
}
func (r *subscriberRepo) FindByBuildID(context.Context, kernel.BuildID) (*model.DeploymentRun, error) {
	if r.findBarrier != nil {
		r.mu.Lock()
		copy := *r.run
		r.findCalls++
		if r.findCalls == 2 {
			close(r.findBarrier)
		}
		r.mu.Unlock()
		<-r.findBarrier
		return &copy, r.findErr
	}
	return r.run, r.findErr
}
func (r *subscriberRepo) ListByProject(context.Context, kernel.ProjectID, int) ([]*model.DeploymentRun, error) {
	return nil, nil
}
func (r *subscriberRepo) ListActive(context.Context) ([]*model.DeploymentRun, error) { return nil, nil }
func (r *subscriberRepo) TryAcquireLease(context.Context, kernel.DeploymentRunID, int64, string, time.Duration) (bool, error) {
	return true, nil
}
func (r *subscriberRepo) ReleaseLease(context.Context, kernel.DeploymentRunID, int64, string) error {
	return nil
}
func (r *subscriberRepo) RecordInfrastructureFailure(_ context.Context, _ kernel.DeploymentRunID, _ int64, message string) (*model.DeploymentRun, error) {
	r.run.AttemptCount++
	if r.run.AttemptCount >= 3 {
		r.run.Fail(message, time.Now())
		r.run.AttemptCount = 3
	}
	r.run.Version++
	return r.run, nil
}
func (r *subscriberRepo) ListEvents(context.Context, kernel.DeploymentRunID) ([]model.DeploymentRunEvent, error) {
	return nil, nil
}
func (*subscriberRepo) AppendEvent(context.Context, *model.DeploymentRun, string) error {
	return nil
}
func (r *subscriberRepo) HasActiveByProject(context.Context, kernel.ProjectID) (bool, error) {
	return false, nil
}

type subscriberPublisher struct {
	calls int
	err   error
	mu    sync.Mutex
}

func (p *subscriberPublisher) PublishRun(context.Context, *model.DeploymentRun) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	return p.err
}

func newCompletedFixture() (*model.DeploymentRun, kernel.BuildID, *subscriberRepo, *subscriberPublisher, *OnBuildCompleted) {
	run := model.NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(), kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")
	run.Status = model.StatusBuilding
	buildID := kernel.NewBuildID()
	run.BuildID = &buildID
	repo := &subscriberRepo{run: run}
	publisher := &subscriberPublisher{}
	return run, buildID, repo, publisher, NewOnBuildCompleted(repo, publisher)
}

func buildEvent(run *model.DeploymentRun, buildID kernel.BuildID, status buildmodel.BuildStatus) deliveryevent.BuildCompleted {
	return deliveryevent.NewBuildCompleted(buildID, run.ProjectID, run.CreatedBy, run.WorkspaceID, "orders", string(status), 1)
}

func TestBuildCompletedFailureFailsRunWithoutPublishingDeploy(t *testing.T) {
	run, buildID, repo, publisher, handler := newCompletedFixture()
	require.NoError(t, handler.Handle(context.Background(), buildEvent(run, buildID, buildmodel.BuildStatusFailed)))
	require.Equal(t, model.StatusFailed, run.Status)
	require.Equal(t, 1, repo.updates)
	require.Zero(t, publisher.calls)
}

func TestBuildCompletedAbortedAbortsRun(t *testing.T) {
	run, buildID, _, publisher, handler := newCompletedFixture()
	require.NoError(t, handler.Handle(context.Background(), buildEvent(run, buildID, buildmodel.BuildStatusAborted)))
	require.Equal(t, model.StatusAborted, run.Status)
	require.Zero(t, publisher.calls)
}

func TestBuildCompletedSuccessPersistsDeployingBeforePublish(t *testing.T) {
	run, buildID, repo, publisher, handler := newCompletedFixture()
	require.NoError(t, handler.Handle(context.Background(), buildEvent(run, buildID, buildmodel.BuildStatusSuccess)))
	require.Equal(t, model.StatusDeploying, run.Status)
	require.Equal(t, 1, repo.updates)
	require.Equal(t, 1, publisher.calls)
}

func TestBuildCompletedDuplicateEventDoesNotRepublishAfterPublishFailure(t *testing.T) {
	run, buildID, _, publisher, handler := newCompletedFixture()
	publisher.err = errors.New("mq unavailable")
	require.Error(t, handler.Handle(context.Background(), buildEvent(run, buildID, buildmodel.BuildStatusSuccess)))
	require.Equal(t, model.StatusDeploying, run.Status)

	publisher.err = nil
	require.NoError(t, handler.Handle(context.Background(), buildEvent(run, buildID, buildmodel.BuildStatusSuccess)))
	require.Equal(t, 1, publisher.calls)
	require.Equal(t, 1, run.AttemptCount)
}

func TestBuildCompletedSequentialDuplicatePublishesExactlyOnce(t *testing.T) {
	run, buildID, _, publisher, handler := newCompletedFixture()
	event := buildEvent(run, buildID, buildmodel.BuildStatusSuccess)

	require.NoError(t, handler.Handle(context.Background(), event))
	require.NoError(t, handler.Handle(context.Background(), event))

	require.Equal(t, 1, publisher.calls)
}

func TestBuildCompletedIgnoresUnrelatedOrOutOfOrderEvent(t *testing.T) {
	run, _, repo, publisher, handler := newCompletedFixture()
	require.NoError(t, handler.Handle(context.Background(), buildEvent(run, kernel.NewBuildID(), buildmodel.BuildStatusSuccess)))
	require.Equal(t, model.StatusBuilding, run.Status)
	require.Zero(t, repo.updates)
	require.Zero(t, publisher.calls)

	run.Status = model.StatusSucceeded
	require.NoError(t, handler.Handle(context.Background(), buildEvent(run, *run.BuildID, buildmodel.BuildStatusSuccess)))
	require.Zero(t, publisher.calls)
}

func TestBuildCompletedIgnoresUnknownFutureBuildStatus(t *testing.T) {
	run, buildID, repo, publisher, handler := newCompletedFixture()
	require.NoError(t, handler.Handle(context.Background(), buildEvent(run, buildID, buildmodel.BuildStatus("future"))))
	require.Equal(t, model.StatusBuilding, run.Status)
	require.Zero(t, repo.updates)
	require.Zero(t, publisher.calls)
}

func TestBuildCompletedConcurrentAbortIsNotOverwritten(t *testing.T) {
	run, buildID, repo, publisher, handler := newCompletedFixture()
	latest := *run
	latest.Status = model.StatusAborted
	repo.afterUpdate = &latest
	repo.updateErr = domain.ErrDeploymentRunConcurrentUpdate

	require.NoError(t, handler.Handle(context.Background(), buildEvent(run, buildID, buildmodel.BuildStatusSuccess)))
	require.Equal(t, model.StatusAborted, repo.run.Status)
	require.Zero(t, publisher.calls)
}

func TestBuildCompletedCASConflictAlreadyDeployingDoesNotPublish(t *testing.T) {
	run, buildID, _, publisher, handler := newCompletedFixture()
	latest := *run
	latest.Status = model.StatusDeploying
	repo := handler.repo.(*subscriberRepo)
	repo.afterUpdate = &latest
	repo.updateErr = domain.ErrDeploymentRunConcurrentUpdate

	require.NoError(t, handler.Handle(context.Background(), buildEvent(run, buildID, buildmodel.BuildStatusSuccess)))

	require.Zero(t, publisher.calls)
}

func TestConcurrentBuildCompletedEventsPublishExactlyOnce(t *testing.T) {
	run := model.NewDeploymentRun(
		kernel.NewWorkspaceID(), kernel.NewProjectID(), kernel.NewClusterInfoID(),
		kernel.NewMemberID(), "branch", "main", "",
	)
	run.Status = model.StatusBuilding
	buildID := kernel.NewBuildID()
	run.BuildID = &buildID
	repo := &subscriberRepo{run: run, cas: true, findBarrier: make(chan struct{})}
	publisher := &subscriberPublisher{}
	handler := NewOnBuildCompleted(repo, publisher)
	event := buildEvent(run, buildID, buildmodel.BuildStatusSuccess)
	errs := make(chan error, 2)

	go func() { errs <- handler.Handle(context.Background(), event) }()
	go func() { errs <- handler.Handle(context.Background(), event) }()

	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.Equal(t, 1, publisher.calls)
	require.Equal(t, model.StatusDeploying, repo.run.Status)
}

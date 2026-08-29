package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/stretchr/testify/require"
)

func TestReconcilerPublishesEveryActiveRunOnStartupScan(t *testing.T) {
	first := newOrchestratorFixture(model.StatusBuilding)
	second := newOrchestratorFixture(model.StatusDeploying)
	repo := &orchestratorRepo{active: []*model.DeploymentRun{first.run, second.run}}
	publisher := &orchestratorPublisher{}

	require.NoError(t, NewReconciler(repo, publisher).RunOnce(context.Background()))

	require.Equal(t, 2, publisher.calls)
}

func TestReconcilerStopsTickerWithContext(t *testing.T) {
	f := newOrchestratorFixture(model.StatusQueued)
	f.repo.active = nil
	ctx, cancel := context.WithCancel(context.Background())
	done := NewReconciler(f.repo, f.publisher).Start(ctx, time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reconciler goroutine did not stop")
	}
}

func TestReconcilerContinuesAfterPerRunPublishFailureAndReportsIt(t *testing.T) {
	first := newOrchestratorFixture(model.StatusBuilding)
	second := newOrchestratorFixture(model.StatusDeploying)
	repo := &orchestratorRepo{active: []*model.DeploymentRun{first.run, second.run}}
	publishErr := errors.New("broker unavailable")
	publisher := &orchestratorPublisher{errs: []error{publishErr, nil}}
	var reported error

	err := NewReconciler(repo, publisher, func(err error) { reported = err }).RunOnce(context.Background())

	require.ErrorIs(t, err, publishErr)
	require.ErrorIs(t, reported, publishErr)
	require.Equal(t, 2, publisher.calls)
}

func TestReconcilerStartReturnsBeforeInitialScanCompletes(t *testing.T) {
	f := newOrchestratorFixture(model.StatusQueued)
	f.repo.listEntered = make(chan struct{})
	f.repo.listRelease = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())

	done := NewReconciler(f.repo, f.publisher).Start(ctx, time.Hour)

	select {
	case <-f.repo.listEntered:
	case <-time.After(time.Second):
		t.Fatal("initial scan did not start")
	}
	cancel()
	close(f.repo.listRelease)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reconciler did not exit")
	}
}

func TestReconcilerPublishFailuresConsumeGlobalRunBudget(t *testing.T) {
	f := newOrchestratorFixture(model.StatusBuilding)
	f.repo.active = nil
	f.publisher.err = errors.New("broker unavailable")
	reconciler := NewReconciler(f.repo, f.publisher)

	require.Error(t, reconciler.RunOnce(context.Background()))
	require.Error(t, reconciler.RunOnce(context.Background()))
	require.Error(t, reconciler.RunOnce(context.Background()))
	require.Equal(t, model.StatusFailed, f.run.Status)
	require.Equal(t, 3, f.run.AttemptCount)
	calls := f.publisher.calls

	require.NoError(t, reconciler.RunOnce(context.Background()))
	require.Equal(t, calls, f.publisher.calls)
}

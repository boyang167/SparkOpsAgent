package consumer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type consumerRepo struct {
	run *model.DeploymentRun
	err error
}

func (*consumerRepo) Create(context.Context, *model.DeploymentRun) error { return nil }
func (*consumerRepo) Update(context.Context, *model.DeploymentRun) error { return nil }
func (r *consumerRepo) FindByID(context.Context, kernel.DeploymentRunID) (*model.DeploymentRun, error) {
	return r.run, r.err
}
func (*consumerRepo) FindByBuildID(context.Context, kernel.BuildID) (*model.DeploymentRun, error) {
	return nil, domain.ErrDeploymentRunNotFound
}
func (*consumerRepo) ListByProject(context.Context, kernel.ProjectID, int) ([]*model.DeploymentRun, error) {
	return nil, nil
}
func (*consumerRepo) ListActive(context.Context) ([]*model.DeploymentRun, error) { return nil, nil }
func (*consumerRepo) TryAcquireLease(context.Context, kernel.DeploymentRunID, int64, string, time.Duration) (bool, error) {
	return true, nil
}
func (*consumerRepo) ReleaseLease(context.Context, kernel.DeploymentRunID, int64, string) error {
	return nil
}
func (*consumerRepo) RecordInfrastructureFailure(context.Context, kernel.DeploymentRunID, int64, string) (*model.DeploymentRun, error) {
	return nil, nil
}
func (*consumerRepo) ListEvents(context.Context, kernel.DeploymentRunID) ([]model.DeploymentRunEvent, error) {
	return nil, nil
}
func (*consumerRepo) HasActiveByProject(context.Context, kernel.ProjectID) (bool, error) {
	return false, nil
}

type countingStepper struct{ calls int }

func (s *countingStepper) Step(context.Context, kernel.DeploymentRunID, kernel.WorkspaceID) error {
	s.calls++
	return nil
}

func TestConsumerAcknowledgesUnknownAndTerminalRuns(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	message := Message{RunID: kernel.NewDeploymentRunID().String(), WorkspaceID: wsID.String()}

	missingStepper := &countingStepper{}
	missingConsumer, err := New(&consumerRepo{err: domain.ErrDeploymentRunNotFound}, missingStepper)
	require.NoError(t, err)
	require.NoError(t, missingConsumer.Handle(context.Background(), message))
	require.Zero(t, missingStepper.calls)

	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")
	run.Fail("stopped", run.CreatedAt)
	terminalStepper := &countingStepper{}
	terminalConsumer, err := New(&consumerRepo{run: run}, terminalStepper)
	require.NoError(t, err)
	require.NoError(t, terminalConsumer.Handle(context.Background(),
		Message{RunID: run.ID.String(), WorkspaceID: wsID.String()}))
	require.Zero(t, terminalStepper.calls)
}

func TestConsumerCallsStepperForActiveRun(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")
	stepper := &countingStepper{}
	consumer, err := New(&consumerRepo{run: run}, stepper)
	require.NoError(t, err)

	err = consumer.Handle(context.Background(),
		Message{RunID: run.ID.String(), WorkspaceID: wsID.String()})

	require.NoError(t, err)
	require.Equal(t, 1, stepper.calls)
}

func TestConsumerRejectsNilStepper(t *testing.T) {
	consumer, err := New(&consumerRepo{}, nil)
	require.Error(t, err)
	require.Nil(t, consumer)
}

type blockingStepper struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (s *blockingStepper) Step(context.Context, kernel.DeploymentRunID, kernel.WorkspaceID) error {
	s.calls.Add(1)
	select {
	case s.started <- struct{}{}:
	default:
	}
	<-s.release
	return nil
}

func TestConsumerGuardsConcurrentRunButAllowsLaterSequentialDelivery(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(),
		kernel.NewMemberID(), "branch", "main", "")
	stepper := &blockingStepper{started: make(chan struct{}, 2), release: make(chan struct{})}
	consumer, err := New(&consumerRepo{run: run}, stepper)
	require.NoError(t, err)
	message := Message{RunID: run.ID.String(), WorkspaceID: wsID.String()}

	firstDone := make(chan error, 1)
	go func() { firstDone <- consumer.Handle(context.Background(), message) }()
	<-stepper.started

	secondDone := make(chan error, 1)
	go func() { secondDone <- consumer.Handle(context.Background(), message) }()
	select {
	case err := <-secondDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("concurrent duplicate did not return")
	}
	require.Equal(t, int32(1), stepper.calls.Load())

	close(stepper.release)
	require.NoError(t, <-firstDone)
	require.NoError(t, consumer.Handle(context.Background(), message))
	require.Equal(t, int32(2), stepper.calls.Load())
}

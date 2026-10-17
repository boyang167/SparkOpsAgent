package service

import (
	"context"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/subscriber"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/consumer"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	deliveryport "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type immediateAssociatedTrigger struct {
	build  *buildmodel.Build
	bus    eventbus.Bus
	status buildmodel.BuildStatus
	calls  int
}

func (t *immediateAssociatedTrigger) TriggerDeploymentAssociated(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	projectID kernel.ProjectID,
	_ deliveryport.BuildRequest,
	beforeStart func(context.Context, *buildmodel.Build) error,
) (*buildmodel.Build, error) {
	t.calls++
	if err := beforeStart(ctx, t.build); err != nil {
		return nil, err
	}
	switch t.status {
	case buildmodel.BuildStatusSuccess:
		t.build.Succeed(1, "team-a/orders", "ok")
	case buildmodel.BuildStatusFailed:
		t.build.Fail("ci failed", "failed")
	}
	err := t.bus.Publish(ctx, deliveryevent.NewBuildCompleted(
		t.build.ID, projectID, memberID, wsID, "orders", string(t.status), 1,
	))
	return t.build, err
}

func runRealQueueClosure(t *testing.T, status buildmodel.BuildStatus) *orchestratorFixture {
	t.Helper()
	f := newOrchestratorFixture(model.StatusQueued)
	queue := messaging.NewNoopQueue()
	t.Cleanup(func() { require.NoError(t, queue.Close()) })
	publisher := consumer.NewPublisher(queue)
	bus := eventbus.NewInMemory()
	trigger := &immediateAssociatedTrigger{
		build: f.trigger.build, bus: bus, status: status,
	}
	f.builds.build = trigger.build
	f.o = NewOrchestrator(
		f.repo, f.builds, f.projects, f.prepare, trigger, f.renderer, f.deployer,
		publisher, 10*time.Minute, 30*time.Minute,
	)
	deployConsumer, err := consumer.New(f.repo, f.o)
	require.NoError(t, err)
	require.NoError(t, deployConsumer.Register(context.Background(), queue))
	completed := subscriber.NewOnBuildCompleted(f.repo, publisher)
	bus.Subscribe(deliveryevent.BuildCompletedName, completed.Handle)

	require.NoError(t, publisher.PublishRun(context.Background(), f.run))
	return f
}

func TestRealNoopQueueAdvancesQueuedAndPreparingWithoutLosingNestedMessage(t *testing.T) {
	f := newOrchestratorFixture(model.StatusQueued)
	queue := messaging.NewNoopQueue()
	t.Cleanup(func() { require.NoError(t, queue.Close()) })
	publisher := consumer.NewPublisher(queue)
	f.o = NewOrchestrator(
		f.repo, f.builds, f.projects, f.prepare, f.trigger, f.renderer, f.deployer,
		publisher, 10*time.Minute, 30*time.Minute,
	)
	deployConsumer, err := consumer.New(f.repo, f.o)
	require.NoError(t, err)
	require.NoError(t, deployConsumer.Register(context.Background(), queue))

	require.NoError(t, publisher.PublishRun(context.Background(), f.run))

	require.Equal(t, model.StatusBuilding, f.run.Status)
	require.Equal(t, 1, f.prepare.calls)
	require.Equal(t, 1, f.trigger.calls)
}

func TestRealQueueAndEventBusCompleteSuccessfulDeployment(t *testing.T) {
	f := runRealQueueClosure(t, buildmodel.BuildStatusSuccess)
	require.Equal(t, model.StatusSucceeded, f.run.Status)
	require.Equal(t, 2, f.deployer.calls)
	require.Equal(t, 1, f.deployer.restartCalls)
}

func TestRealQueueAndEventBusCIShutdownHasZeroApply(t *testing.T) {
	f := runRealQueueClosure(t, buildmodel.BuildStatusFailed)
	require.Equal(t, model.StatusFailed, f.run.Status)
	require.Equal(t, 1, f.deployer.calls)
	require.Zero(t, f.deployer.restartCalls)
}

func TestReconcilerRecoversMissedBuildEventAndDeployingMessage(t *testing.T) {
	for _, initial := range []model.Status{model.StatusBuilding, model.StatusDeploying} {
		t.Run(string(initial), func(t *testing.T) {
			f := newOrchestratorFixture(initial)
			buildID := f.builds.build.ID
			f.run.BuildID = &buildID
			f.builds.build.Succeed(1, "team-a/orders", "ok")
			queue := messaging.NewNoopQueue()
			t.Cleanup(func() { require.NoError(t, queue.Close()) })
			publisher := consumer.NewPublisher(queue)
			f.o = NewOrchestrator(
				f.repo, f.builds, f.projects, f.prepare, f.trigger, f.renderer, f.deployer,
				publisher, 10*time.Minute, 30*time.Minute,
			)
			deployConsumer, err := consumer.New(f.repo, f.o)
			require.NoError(t, err)
			require.NoError(t, deployConsumer.Register(context.Background(), queue))

			require.NoError(t, NewReconciler(f.repo, publisher).RunOnce(context.Background()))

			require.Equal(t, model.StatusSucceeded, f.run.Status)
			require.Equal(t, 1, f.deployer.calls)
		})
	}
}

func TestRealQueueAcknowledgesPermanentBindingErrorAsFailedRun(t *testing.T) {
	f := newOrchestratorFixture(model.StatusQueued)
	f.projects.err = domain.ErrDeploymentBindingInvalid
	queue := messaging.NewNoopQueue()
	t.Cleanup(func() { require.NoError(t, queue.Close()) })
	publisher := consumer.NewPublisher(queue)
	f.o = NewOrchestrator(
		f.repo, f.builds, f.projects, f.prepare, f.trigger, f.renderer, f.deployer,
		publisher, 10*time.Minute, 30*time.Minute,
	)
	deployConsumer, err := consumer.New(f.repo, f.o)
	require.NoError(t, err)
	require.NoError(t, deployConsumer.Register(context.Background(), queue))

	require.NoError(t, publisher.PublishRun(context.Background(), f.run))

	require.Equal(t, model.StatusFailed, f.run.Status)
	require.Equal(t, 1, f.repo.updates)
	require.Zero(t, f.trigger.calls)
}

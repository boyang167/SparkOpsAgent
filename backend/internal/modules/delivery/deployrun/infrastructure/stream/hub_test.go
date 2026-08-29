package stream

import (
	"strings"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestHubUnsubscribeRemovesAndClosesSubscriber(t *testing.T) {
	hub := NewHub()
	runID := kernel.NewDeploymentRunID()
	hub.Publish(runID, "queued\n")

	ch, initial, done, unsubscribe := hub.Subscribe(runID)
	require.Equal(t, "queued\n", initial)
	require.False(t, done)
	unsubscribe()

	_, open := <-ch
	require.False(t, open)
	require.Empty(t, hub.chans[runID.String()])
	hub.Publish(runID, "preparing\n")
}

func TestHubCompleteClosesSubscribersAndCleansRunState(t *testing.T) {
	hub := NewHub()
	runID := kernel.NewDeploymentRunID()
	hub.Publish(runID, "queued\n")
	ch, _, _, unsubscribe := hub.Subscribe(runID)
	defer unsubscribe()

	hub.Complete(runID, "queued\nfailed\n")

	_, open := <-ch
	require.False(t, open)
	require.NotContains(t, hub.chans, runID.String())
	require.NotContains(t, hub.outputs, runID.String())
}

func TestHubPublishEventStreamsStatusAndCompletesTerminalRun(t *testing.T) {
	hub := NewHub()
	run := model.NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(), kernel.NewClusterInfoID(),
		kernel.NewMemberID(), "branch", "main", "")
	ch, _, _, unsubscribe := hub.Subscribe(run.ID)
	defer unsubscribe()

	event := model.DeploymentRunEvent{
		ID: 7, RunID: run.ID, At: time.Date(2026, 8, 29, 1, 2, 3, 0, time.UTC),
		Status: model.StatusQueued, Message: "deployment queued",
	}
	hub.PublishEvent(run, event)
	require.Contains(t, <-ch, `"status":"queued"`)

	run.Fail("ci failed", event.At.Add(time.Second))
	event.ID = 8
	event.Status = model.StatusFailed
	event.Message = "ci failed"
	hub.PublishEvent(run, event)
	terminal := <-ch
	require.True(t, strings.Contains(terminal, `"status":"failed"`))
	_, open := <-ch
	require.False(t, open)
}

func TestHubOverflowDisconnectsAndRemovesSlowSubscriber(t *testing.T) {
	hub := NewHub()
	runID := kernel.NewDeploymentRunID()
	ch, _, _, unsubscribe := hub.Subscribe(runID)
	defer unsubscribe()

	for index := 0; index < subscriberBufferSize+1; index++ {
		hub.Publish(runID, "event\n")
	}

	require.Empty(t, hub.chans[runID.String()])
	for range ch {
	}
}

func TestHubLateSubscriberConsumesCompletedState(t *testing.T) {
	hub := NewHub()
	runID := kernel.NewDeploymentRunID()
	hub.Publish(runID, "queued\n")
	hub.Complete(runID, "failed\n")

	ch, initial, done, unsubscribe := hub.Subscribe(runID)
	defer unsubscribe()

	require.True(t, done)
	require.Contains(t, initial, "queued\n")
	_, open := <-ch
	require.False(t, open)

	_, _, secondDone, secondUnsubscribe := hub.Subscribe(runID)
	defer secondUnsubscribe()
	require.False(t, secondDone, "completed hub state is consumed by the first late subscriber")
}

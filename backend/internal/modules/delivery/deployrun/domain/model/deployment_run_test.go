package model_test

import (
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestDefaultImageTagStripsHyphensAndTakesFirst12(t *testing.T) {
	id, err := kernel.ParseDeploymentRunID("550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	require.Equal(t, "550e8400e29b", model.DefaultImageTag(id))
}

func TestNewDeploymentRunUsesDefaultImageTagWhenEmpty(t *testing.T) {
	run := model.NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(),
		kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")
	require.Equal(t, model.DefaultImageTag(run.ID), run.ImageTag)
	require.Len(t, run.ImageTag, 12)
}

func TestNewDeploymentRunKeepsExplicitImageTag(t *testing.T) {
	run := model.NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(),
		kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "release-1")
	require.Equal(t, "release-1", run.ImageTag)
}

func TestDeploymentRunTransitionRejectsSkippedAndTerminalTransitions(t *testing.T) {
	run := model.NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(),
		kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "release-1")
	require.ErrorIs(t, run.Transition(model.StatusSucceeded, time.Now()), domain.ErrInvalidDeploymentTransition)
	require.NoError(t, run.Transition(model.StatusPreparing, time.Now()))
	require.NoError(t, run.Transition(model.StatusFailed, time.Now()))
	require.ErrorIs(t, run.Transition(model.StatusBuilding, time.Now()), domain.ErrInvalidDeploymentTransition)
}

func TestDeploymentRunHappyPathSetsStagesAndTimestamps(t *testing.T) {
	now := time.Date(2026, 8, 29, 1, 2, 3, 0, time.UTC)
	run := model.NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(),
		kernel.NewClusterInfoID(), kernel.NewMemberID(), "tag", "v1.2.3", "v1.2.3")
	require.Equal(t, model.StatusQueued, run.Status)
	require.NoError(t, run.Transition(model.StatusPreparing, now))
	require.Equal(t, model.StagePreparing, run.CurrentStage)
	require.NotNil(t, run.StartedAt)
	require.NoError(t, run.Transition(model.StatusBuilding, now.Add(time.Second)))
	require.NoError(t, run.Transition(model.StatusDeploying, now.Add(2*time.Second)))
	require.NoError(t, run.Transition(model.StatusSucceeded, now.Add(3*time.Second)))
	require.NotNil(t, run.FinishedAt)
}

func TestDeploymentRunFailDoesNotMutateTerminalRun(t *testing.T) {
	now := time.Date(2026, 8, 29, 2, 3, 4, 0, time.UTC)
	run := model.NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(),
		kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "release-1")
	require.NoError(t, run.Transition(model.StatusPreparing, now))
	require.NoError(t, run.Transition(model.StatusBuilding, now.Add(time.Second)))
	require.NoError(t, run.Transition(model.StatusDeploying, now.Add(2*time.Second)))
	require.NoError(t, run.Transition(model.StatusSucceeded, now.Add(3*time.Second)))

	before := *run
	run.Fail("late failure", now.Add(4*time.Second))

	require.Equal(t, before, *run)
}

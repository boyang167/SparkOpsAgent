package model_test

import (
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestDefaultImageTagUsesGitRefValue(t *testing.T) {
	require.Equal(t, "main", model.DefaultImageTag("main"))
	require.Equal(t, "v1.2.3", model.DefaultImageTag("v1.2.3"))
}

func TestDefaultImageTagSanitizesGitRefForDockerTag(t *testing.T) {
	require.Equal(t, "feature-login", model.DefaultImageTag("feature/login"))
	require.Equal(t, "release-1", model.DefaultImageTag("refs/heads/release-1"))
	require.Equal(t, "v1.0.0", model.DefaultImageTag("refs/tags/v1.0.0"))
}

func TestDefaultImageTagFallsBackToLatestWhenEmpty(t *testing.T) {
	require.Equal(t, "latest", model.DefaultImageTag(""))
	require.Equal(t, "latest", model.DefaultImageTag("   "))
}

func TestNewDeploymentRunUsesGitRefAsDefaultImageTag(t *testing.T) {
	run := model.NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(),
		kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")
	require.Equal(t, "main", run.ImageTag)
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

func TestNewRollbackDeploymentRunCopiesCrossCluster(t *testing.T) {
	wsID, projectID := kernel.NewWorkspaceID(), kernel.NewProjectID()
	target := model.NewDeploymentRun(wsID, projectID, kernel.NewClusterInfoID(),
		kernel.NewMemberID(), "branch", "main", "v1")
	target.CrossCluster = true
	rollback := model.NewRollbackDeploymentRun(wsID, projectID, kernel.NewMemberID(), target)
	require.True(t, rollback.CrossCluster)
	require.True(t, rollback.IsRollback())
	require.True(t, rollback.SkipsBuild())
}

func TestNewEnvironmentSyncDeploymentRunSkipsBuildAndTargetsEnvironment(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	projectID := kernel.NewProjectID()
	clusterID := kernel.NewClusterInfoID()
	actorID := kernel.NewMemberID()
	run := model.NewEnvironmentSyncDeploymentRun(wsID, projectID, clusterID, actorID)
	require.Equal(t, model.RunTypeEnvironmentSync, run.RunType)
	require.False(t, run.CrossCluster)
	require.Equal(t, model.StatusQueued, run.Status)
	require.True(t, run.SkipsBuild())
	require.True(t, run.IsEnvironmentSync())
}

func TestNewSyncDeploymentRunSkipsBuildAndForcesCrossCluster(t *testing.T) {
	run := model.NewSyncDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(),
		kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "v9")
	require.True(t, run.CrossCluster)
	require.True(t, run.IsSyncOnly())
	require.True(t, run.SkipsBuild())
	require.Equal(t, model.RunTypeSync, run.RunType)
	require.Equal(t, "v9", run.ImageTag)
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

package model_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func succeededRun(projectID kernel.ProjectID, clusterID kernel.ClusterInfoID) *model.DeploymentRun {
	run := model.NewDeploymentRun(kernel.NewWorkspaceID(), projectID, clusterID, kernel.NewMemberID(), "branch", "main", "v1")
	run.Status = model.StatusSucceeded
	return run
}

func TestNewUndeployRunCopiesBaselineTarget(t *testing.T) {
	envID := kernel.NewEnvironmentID()
	baseline := succeededRun(kernel.NewProjectID(), kernel.NewClusterInfoID())
	baseline.EnvironmentID = &envID
	baseline.DeployNamespace = "team-a-dev"
	baseline.ImageRepository = "registry.example/team-a/orders"
	actor := kernel.NewMemberID()

	run := model.NewUndeployRun(baseline.WorkspaceID, baseline.ProjectID, actor, baseline, model.UndeployOptions{DeleteConfig: true})

	require.Equal(t, model.RunTypeUndeploy, run.RunType)
	require.Equal(t, model.StatusQueued, run.Status)
	require.Equal(t, baseline.ClusterID, run.ClusterID)
	require.Equal(t, &envID, run.EnvironmentID)
	require.Equal(t, "team-a-dev", run.DeployNamespace)
	require.Equal(t, "v1", run.ImageTag)
	require.Equal(t, "registry.example/team-a/orders", run.ImageRepository)
	require.Equal(t, actor, run.CreatedBy)
	require.False(t, run.CrossCluster)
	require.NotEqual(t, baseline.ID, run.ID)
	require.Equal(t, &model.UndeployOptions{DeleteConfig: true}, run.UndeployOptions)
	require.True(t, run.IsUndeploy())
	require.True(t, run.SkipsBuild())
}

func TestOnlineClustersUsesLatestRunPerCluster(t *testing.T) {
	projectID := kernel.NewProjectID()
	dev := kernel.NewClusterInfoID()
	uat := kernel.NewClusterInfoID()
	prod := kernel.NewClusterInfoID()

	devUndeploy := model.NewUndeployRun(kernel.NewWorkspaceID(), projectID, kernel.NewMemberID(), succeededRun(projectID, dev), model.UndeployOptions{})
	devUndeploy.Status = model.StatusSucceeded
	uatFailed := succeededRun(projectID, uat)
	uatFailed.Status = model.StatusFailed

	runs := []*model.DeploymentRun{
		devUndeploy,
		succeededRun(projectID, prod),
		uatFailed,
		succeededRun(projectID, dev),
		succeededRun(projectID, uat),
	}

	require.Equal(t, []kernel.ClusterInfoID{prod}, model.OnlineClusters(runs))
	require.Len(t, model.LatestRunByCluster(runs), 3)
	require.False(t, model.IsOnlineRun(devUndeploy))
	require.False(t, model.IsOnlineRun(nil))
}

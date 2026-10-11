package command

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestResolveRollbackTargetSkipsUndeployRuns(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	projectID := kernel.NewProjectID()
	clusterID := kernel.NewClusterInfoID()
	older := model.NewDeploymentRun(wsID, projectID, clusterID, kernel.NewMemberID(), "branch", "main", "v1")
	older.Status = model.StatusSucceeded
	newer := model.NewDeploymentRun(wsID, projectID, clusterID, kernel.NewMemberID(), "branch", "main", "v2")
	newer.Status = model.StatusSucceeded
	undeploy := model.NewUndeployRun(wsID, projectID, kernel.NewMemberID(), newer, model.UndeployOptions{})
	undeploy.Status = model.StatusSucceeded

	current, target, err := ResolveRollbackTarget([]*model.DeploymentRun{undeploy, newer, older}, wsID, projectID, "")

	require.NoError(t, err)
	require.Equal(t, newer.ID, current.ID)
	require.Equal(t, older.ID, target.ID)
}

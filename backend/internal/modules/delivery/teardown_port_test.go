package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type teardownRunLister struct {
	runs   []*model.DeploymentRun
	active bool
}

func (l teardownRunLister) ListByProject(context.Context, kernel.ProjectID, int) ([]*model.DeploymentRun, error) {
	return l.runs, nil
}

func (l teardownRunLister) HasActiveByProject(context.Context, kernel.ProjectID) (bool, error) {
	return l.active, nil
}

func TestTeardownPortOnlineClustersFiltersWorkspace(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	projectID := kernel.NewProjectID()
	own := model.NewDeploymentRun(wsID, projectID, kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "v1")
	own.Status = model.StatusSucceeded
	foreign := model.NewDeploymentRun(kernel.NewWorkspaceID(), projectID, kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "v1")
	foreign.Status = model.StatusSucceeded
	foreign.CreatedAt = own.CreatedAt.Add(time.Second)

	teardown := &teardownPort{runs: teardownRunLister{runs: []*model.DeploymentRun{foreign, own}}}
	clusters, err := teardown.OnlineClusters(context.Background(), wsID, projectID)

	require.NoError(t, err)
	require.Equal(t, []kernel.ClusterInfoID{own.ClusterID}, clusters)
}

func TestTeardownPortHasActiveRun(t *testing.T) {
	teardown := &teardownPort{runs: teardownRunLister{active: true}}

	active, err := teardown.HasActiveRun(context.Background(), kernel.NewProjectID())

	require.NoError(t, err)
	require.True(t, active)
}

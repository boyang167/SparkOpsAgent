package delivery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
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

func (l teardownRunLister) FindByID(_ context.Context, runID kernel.DeploymentRunID) (*model.DeploymentRun, error) {
	for _, run := range l.runs {
		if run.ID == runID {
			return run, nil
		}
	}
	return nil, errors.New("run not found")
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

func TestTeardownPortLoadsArchiveGenerationByRunID(t *testing.T) {
	run := model.NewDeploymentRun(
		kernel.NewWorkspaceID(), kernel.NewProjectID(), kernel.NewClusterInfoID(),
		kernel.NewMemberID(), "branch", "main", "v1",
	)
	run.ArchiveGeneration = uuid.NewString()
	teardown := &teardownPort{runs: teardownRunLister{runs: []*model.DeploymentRun{run}}}

	generation, err := teardown.ArchiveGeneration(context.Background(), run.ID)

	require.NoError(t, err)
	require.Equal(t, run.ArchiveGeneration, generation)
}

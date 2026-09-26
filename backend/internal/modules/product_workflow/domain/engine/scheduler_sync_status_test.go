package engine

import (
	"context"
	"testing"
	"time"

	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type stubStatusReader struct {
	buildStatus string
}

func (s stubStatusReader) BuildStatus(_ context.Context, _ kernel.BuildID) (string, bool, error) {
	return s.buildStatus, true, nil
}

func (s stubStatusReader) DeployStatus(_ context.Context, _ kernel.DeploymentRunID) (string, bool, error) {
	return "", false, nil
}

func TestSyncRunExternalStatuses_skipsWhenRunLockBusy(t *testing.T) {
	runID := kernel.NewProductWorkflowRunID()
	buildID := kernel.NewBuildID()
	repo := &concurrentRunRepo{
		run: &model.ProductWorkflowRun{
			ID: runID, Status: model.RunStatusRunning,
			StageRuns: []model.StageRun{{
				Jobs: []model.JobRun{{
					Builds: []model.ProjectBuildRun{{
						BuildID: buildID.String(),
						Status:  model.JobRunStatusRunning,
					}},
				}},
			}},
		},
	}
	scheduler := NewScheduler(repo, nil, nil, nil, nil)
	scheduler.SetStatusReader(stubStatusReader{buildStatus: string(buildmodel.BuildStatusSuccess)})

	unlock := scheduler.locker.lock(runID)
	done := make(chan struct{})
	go func() {
		require.NoError(t, scheduler.SyncRunExternalStatuses(context.Background(), runID))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("SyncRunExternalStatuses blocked while run lock is held")
	}

	unlock()
	require.NoError(t, scheduler.SyncRunExternalStatuses(context.Background(), runID))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, model.JobRunStatusSuccess, repo.run.StageRuns[0].Jobs[0].Builds[0].Status)
}

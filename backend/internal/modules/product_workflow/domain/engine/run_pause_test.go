package engine

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestPauseRun_and_ResumeRun_userPause(t *testing.T) {
	runID := kernel.NewProductWorkflowRunID()
	wfID := kernel.NewProductWorkflowID()
	run := &model.ProductWorkflowRun{
		ID: runID, WorkflowID: wfID, Status: model.RunStatusRunning, CurrentStage: 0,
		StageRuns: []model.StageRun{{
			Name: "build", Status: model.JobRunStatusRunning,
			Jobs: []model.JobRun{{
				Name: "build-1", Type: model.JobTypeSparkopsBuild, Status: model.JobRunStatusSuccess,
			}},
		}},
	}
	repo := &concurrentRunRepo{run: run, wf: &model.ProductWorkflow{
		ID: wfID,
		Stages: []model.WorkflowStage{{
			Name: "build",
			Jobs: []model.WorkflowJob{{Name: "build-1", Type: model.JobTypeSparkopsBuild}},
		}},
	}}
	scheduler := NewScheduler(repo, nil, nil, nil, nil, nil)

	require.NoError(t, scheduler.PauseRun(context.Background(), runID))
	repo.mu.Lock()
	require.Equal(t, model.RunStatusPaused, repo.run.Status)
	require.True(t, IsUserPaused(repo.run))
	repo.mu.Unlock()

	require.NoError(t, scheduler.ResumeRun(context.Background(), runID))
	repo.mu.Lock()
	require.Equal(t, model.RunStatusSuccess, repo.run.Status)
	repo.mu.Unlock()
}

func TestPauseRun_rejectsNonRunning(t *testing.T) {
	runID := kernel.NewProductWorkflowRunID()
	run := &model.ProductWorkflowRun{ID: runID, Status: model.RunStatusSuccess}
	repo := &concurrentRunRepo{run: run}
	scheduler := NewScheduler(repo, nil, nil, nil, nil, nil)
	require.ErrorIs(t, scheduler.PauseRun(context.Background(), runID), domain.ErrRunNotPausable)
}

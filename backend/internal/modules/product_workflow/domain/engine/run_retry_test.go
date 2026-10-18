package engine

import (
	"context"
	"testing"
	"time"

	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type recordingBuildTrigger struct {
	requests []appctx.AutomatedBuildRequest
	ids      []kernel.BuildID
}

func (t *recordingBuildTrigger) TriggerAutomated(context.Context, kernel.ProjectID, string) (kernel.BuildID, error) {
	return kernel.NewBuildID(), nil
}

func (t *recordingBuildTrigger) TriggerAutomatedBuild(_ context.Context, req appctx.AutomatedBuildRequest) (kernel.BuildID, error) {
	id := kernel.NewBuildID()
	t.requests = append(t.requests, req)
	t.ids = append(t.ids, id)
	return id, nil
}

func retryFixture(t *testing.T) (*concurrentRunRepo, *recordingBuildTrigger, *Scheduler, string, string) {
	t.Helper()
	p1 := kernel.NewProjectID().String()
	p2 := kernel.NewProjectID().String()
	wfID := kernel.NewProductWorkflowID()
	run := &model.ProductWorkflowRun{
		ID: kernel.NewProductWorkflowRunID(), WorkflowID: wfID, Status: model.RunStatusFailed,
		ErrorMessage: "build failed", CompletedAt: ptrTime(time.Now()),
		StageRuns: []model.StageRun{
			{
				Name: "build", Status: model.JobRunStatusFailed,
				Jobs: []model.JobRun{{
					Name: "build-1", Type: model.JobTypeSparkopsBuild, Status: model.JobRunStatusFailed,
					ErrorMessage: "build job failed",
					Builds: []model.ProjectBuildRun{
						{ProjectID: p1, BuildID: kernel.NewBuildID().String(), Status: model.JobRunStatusSuccess},
						{ProjectID: p2, BuildID: kernel.NewBuildID().String(), Status: model.JobRunStatusFailed, ErrorMessage: "build failed"},
					},
				}},
			},
			{
				Name: "notify", Status: model.JobRunStatusFailed,
				Jobs: []model.JobRun{{Name: "notify-1", Type: model.JobTypeFreestyle, Status: model.JobRunStatusFailed, ErrorMessage: "用户终止"}},
			},
		},
	}
	repo := &concurrentRunRepo{
		run: run,
		wf: &model.ProductWorkflow{
			ID: wfID,
			Stages: []model.WorkflowStage{
				{Name: "build", Jobs: []model.WorkflowJob{{
					Name: "build-1", Type: model.JobTypeSparkopsBuild,
					Spec: map[string]any{"project_builds": []any{
						map[string]any{"project_id": p1, "branch": "main"},
						map[string]any{"project_id": p2, "branch": "main"},
					}},
				}}},
				{Name: "notify", Jobs: []model.WorkflowJob{{Name: "notify-1", Type: model.JobTypeFreestyle}}},
			},
		},
		refs: map[string]*model.WorkflowBuildRef{},
	}
	trigger := &recordingBuildTrigger{}
	scheduler := NewScheduler(repo, NewBuildExecutor(trigger, repo, nil), nil, nil, nil, nil)
	return repo, trigger, scheduler, p1, p2
}

func ptrTime(value time.Time) *time.Time { return &value }

func TestRetryRunRebuildsOnlyFailedProjectsAndCompletes(t *testing.T) {
	repo, trigger, scheduler, p1, p2 := retryFixture(t)
	runID := repo.run.ID

	require.NoError(t, scheduler.RetryRun(context.Background(), runID))

	require.Len(t, trigger.requests, 1)
	require.Equal(t, p2, trigger.requests[0].ProjectID.String())
	run, _ := repo.FindRunByID(context.Background(), runID)
	require.Equal(t, model.RunStatusRunning, run.Status)
	require.Empty(t, run.ErrorMessage)
	require.Nil(t, run.CompletedAt)
	job := run.StageRuns[0].Jobs[0]
	require.Equal(t, model.JobRunStatusRunning, job.Status)
	require.Len(t, job.Builds, 2)
	require.Equal(t, p1, job.Builds[0].ProjectID)
	require.Equal(t, model.JobRunStatusSuccess, job.Builds[0].Status)
	require.Equal(t, model.JobRunStatusRunning, job.Builds[1].Status)
	require.Equal(t, model.JobRunStatusPending, run.StageRuns[1].Status)
	require.Empty(t, run.StageRuns[1].Jobs[0].ErrorMessage)

	unlock := scheduler.locker.lock(runID)
	err := scheduler.handleBuildCompletedLocked(context.Background(), runID, "build-1", trigger.ids[0], string(buildmodel.BuildStatusSuccess))
	unlock()
	require.NoError(t, err)

	run, _ = repo.FindRunByID(context.Background(), runID)
	require.Equal(t, model.RunStatusSuccess, run.Status)
}

func TestRetryRunRejectsNonFailedRun(t *testing.T) {
	repo, _, scheduler, _, _ := retryFixture(t)
	repo.run.Status = model.RunStatusRunning

	err := scheduler.RetryRun(context.Background(), repo.run.ID)
	require.ErrorIs(t, err, domain.ErrRunNotRetryable)
}

func TestRetryRunRejectsChangedDefinition(t *testing.T) {
	repo, trigger, scheduler, _, _ := retryFixture(t)
	repo.wf.Stages[0].Jobs[0].Name = "build-renamed"

	err := scheduler.RetryRun(context.Background(), repo.run.ID)
	require.ErrorIs(t, err, domain.ErrRunDefinitionChanged)
	require.Empty(t, trigger.requests)
	require.Equal(t, model.RunStatusFailed, repo.run.Status)
}

func TestResetRunForRetryKeepsFinishedStages(t *testing.T) {
	run := &model.ProductWorkflowRun{
		Status: model.RunStatusFailed, CurrentStage: 1, ErrorMessage: "deployment failed",
		StageRuns: []model.StageRun{
			{Name: "build", Status: model.JobRunStatusSuccess, Jobs: []model.JobRun{{Name: "b", Status: model.JobRunStatusSuccess}}},
			{Name: "deploy", Status: model.JobRunStatusFailed, Jobs: []model.JobRun{
				{Name: "ok", Status: model.JobRunStatusSuccess},
				{Name: "d", Status: model.JobRunStatusFailed, ErrorMessage: "deploy job failed", Deploys: []model.ProjectDeployRun{
					{ProjectID: "a", DeploymentRunID: "x", Status: model.JobRunStatusSuccess},
					{ProjectID: "b", DeploymentRunID: "y", Status: model.JobRunStatusFailed},
				}},
			}},
		},
	}
	now := time.Now().UTC()

	ResetRunForRetry(run, 1, now)

	require.Equal(t, model.JobRunStatusSuccess, run.StageRuns[0].Status)
	require.Equal(t, model.JobRunStatusPending, run.StageRuns[1].Status)
	require.Equal(t, model.JobRunStatusSuccess, run.StageRuns[1].Jobs[0].Status)
	failed := run.StageRuns[1].Jobs[1]
	require.Equal(t, model.JobRunStatusPending, failed.Status)
	require.Empty(t, failed.ErrorMessage)
	require.Equal(t, []model.ProjectDeployRun{{ProjectID: "a", DeploymentRunID: "x", Status: model.JobRunStatusSuccess}}, failed.Deploys)
	require.Equal(t, 1, run.CurrentStage)
	require.Equal(t, model.RunStatusRunning, run.Status)
	require.Equal(t, &now, run.StartedAt)
}

func TestHandleBuildFailedMarksJobAndStageFailed(t *testing.T) {
	buildID := kernel.NewBuildID()
	runID := kernel.NewProductWorkflowRunID()
	wfID := kernel.NewProductWorkflowID()
	repo := &concurrentRunRepo{
		run: &model.ProductWorkflowRun{
			ID: runID, WorkflowID: wfID, Status: model.RunStatusRunning,
			StageRuns: []model.StageRun{{
				Name: "build", Status: model.JobRunStatusRunning,
				Jobs: []model.JobRun{{
					Name: "build-1", Type: model.JobTypeSparkopsBuild, Status: model.JobRunStatusRunning,
					Builds: []model.ProjectBuildRun{{BuildID: buildID.String(), Status: model.JobRunStatusRunning}},
				}},
			}},
		},
		wf: &model.ProductWorkflow{ID: wfID, Stages: []model.WorkflowStage{
			{Name: "build", Jobs: []model.WorkflowJob{{Name: "build-1", Type: model.JobTypeSparkopsBuild}}},
		}},
		refs: map[string]*model.WorkflowBuildRef{buildID.String(): {BuildID: buildID, RunID: runID, JobName: "build-1"}},
	}
	scheduler := NewScheduler(repo, nil, nil, nil, nil, nil)

	require.NoError(t, scheduler.HandleBuildCompleted(context.Background(), buildID, string(buildmodel.BuildStatusFailed)))

	require.Equal(t, model.RunStatusFailed, repo.run.Status)
	require.Equal(t, model.JobRunStatusFailed, repo.run.StageRuns[0].Jobs[0].Status)
	require.Equal(t, model.JobRunStatusFailed, repo.run.StageRuns[0].Status)
}

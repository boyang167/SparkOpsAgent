package engine

import (
	"context"
	"testing"

	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type stubDeployTrigger struct{}

func (stubDeployTrigger) TriggerForEnvironment(context.Context, appctx.WorkflowDeploymentRequest) (kernel.DeploymentRunID, error) {
	return kernel.NewDeploymentRunID(), nil
}

func TestProgressStuckRun_advancesToDeployAfterBuildSuccess(t *testing.T) {
	buildA := kernel.NewBuildID()
	buildB := kernel.NewBuildID()
	runID := kernel.NewProductWorkflowRunID()
	wfID := kernel.NewProductWorkflowID()
	projectA := kernel.NewProjectID()
	projectB := kernel.NewProjectID()

	run := &model.ProductWorkflowRun{
		ID: runID, WorkflowID: wfID, Status: model.RunStatusRunning, CurrentStage: 0,
		StageRuns: []model.StageRun{
			{
				Name: "build", Status: model.JobRunStatusRunning,
				Jobs: []model.JobRun{{
					Name: "build-1", Type: model.JobTypeSparkopsBuild, Status: model.JobRunStatusRunning,
					Builds: []model.ProjectBuildRun{
						{BuildID: buildA.String(), ProjectID: projectA.String(), Status: model.JobRunStatusSuccess, Branch: "main"},
						{BuildID: buildB.String(), ProjectID: projectB.String(), Status: model.JobRunStatusSuccess, Branch: "main"},
					},
				}},
			},
			{
				Name: "deploy", Status: model.JobRunStatusPending,
				Jobs: []model.JobRun{{
					Name: "deploy-2", Type: model.JobTypeSparkopsDeploy, Status: model.JobRunStatusPending,
				}},
			},
		},
	}
	envID := kernel.NewEnvironmentID()
	repo := &concurrentRunRepo{
		run: run,
		wf: &model.ProductWorkflow{
			ID: wfID,
			Stages: []model.WorkflowStage{
				{Name: "build", Jobs: []model.WorkflowJob{{Name: "build-1", Type: model.JobTypeSparkopsBuild}}},
				{Name: "deploy", Jobs: []model.WorkflowJob{{
					Name: "deploy-2", Type: model.JobTypeSparkopsDeploy,
					Spec: map[string]any{
						"environment_id": envID.String(),
						"source":         "fromjob",
						"job_name":       "build-1",
					},
				}}},
			},
		},
	}
	deploys := NewDeployExecutor(stubDeployTrigger{}, nil, nil, repo)
	scheduler := NewScheduler(repo, nil, nil, deploys, nil)
	scheduler.SetStatusReader(stubStatusReader{buildStatus: string(buildmodel.BuildStatusSuccess)})

	require.NoError(t, scheduler.ProgressStuckRun(context.Background(), runID))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, 1, repo.run.CurrentStage)
	require.Equal(t, model.JobRunStatusRunning, repo.run.StageRuns[1].Jobs[0].Status)
	require.NotEmpty(t, repo.run.StageRuns[1].Jobs[0].Deploys)
}

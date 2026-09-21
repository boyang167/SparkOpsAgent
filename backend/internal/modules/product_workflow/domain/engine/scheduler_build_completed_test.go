package engine

import (
	"context"
	"sync"
	"testing"

	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type concurrentRunRepo struct {
	mu  sync.Mutex
	run *model.ProductWorkflowRun
	wf  *model.ProductWorkflow
	refs map[string]*model.WorkflowBuildRef
}

func (r *concurrentRunRepo) UpdateRun(_ context.Context, run *model.ProductWorkflowRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.run = cloneRun(run)
	return nil
}

func (r *concurrentRunRepo) FindRunByID(_ context.Context, _ kernel.ProductWorkflowRunID) (*model.ProductWorkflowRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneRun(r.run), nil
}

func (r *concurrentRunRepo) FindWorkflowByID(_ context.Context, _ kernel.ProductWorkflowID) (*model.ProductWorkflow, error) {
	return r.wf, nil
}

func (r *concurrentRunRepo) FindRunByBuildID(_ context.Context, buildID kernel.BuildID) (*model.WorkflowBuildRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ref, ok := r.refs[buildID.String()]
	if !ok {
		return nil, domain.ErrProductWorkflowRunNotFound
	}
	return ref, nil
}

func (r *concurrentRunRepo) SaveWorkflow(context.Context, *model.ProductWorkflow) error { return nil }
func (r *concurrentRunRepo) UpdateWorkflow(context.Context, *model.ProductWorkflow) error {
	return nil
}
func (r *concurrentRunRepo) DeleteWorkflow(context.Context, kernel.ProductWorkflowID) error { return nil }
func (r *concurrentRunRepo) ListWorkflowsByProduct(context.Context, kernel.ProductID) ([]*model.ProductWorkflow, error) {
	return nil, nil
}
func (r *concurrentRunRepo) ListWorkflowsByWorkspace(context.Context, kernel.WorkspaceID) ([]*model.ProductWorkflow, error) {
	return nil, nil
}
func (r *concurrentRunRepo) ListAllWorkflows(context.Context) ([]*model.ProductWorkflow, error) {
	return nil, nil
}
func (r *concurrentRunRepo) CountActiveRuns(context.Context, kernel.ProductWorkflowID) (int64, error) {
	return 0, nil
}
func (r *concurrentRunRepo) ListActiveRuns(context.Context, int) ([]*model.ProductWorkflowRun, error) {
	return nil, nil
}
func (r *concurrentRunRepo) SaveRun(context.Context, *model.ProductWorkflowRun) error { return nil }
func (r *concurrentRunRepo) ListRunsByWorkflow(context.Context, kernel.ProductWorkflowID, int) ([]*model.ProductWorkflowRun, error) {
	return nil, nil
}
func (r *concurrentRunRepo) CountRunsByWorkflow(context.Context, kernel.ProductWorkflowID) (int64, error) {
	return 0, nil
}
func (r *concurrentRunRepo) SaveBuildRef(context.Context, *model.WorkflowBuildRef) error { return nil }
func (r *concurrentRunRepo) SaveDeployRef(context.Context, *model.WorkflowDeployRef) error { return nil }
func (r *concurrentRunRepo) FindRunByDeploymentID(context.Context, kernel.DeploymentRunID) (*model.WorkflowDeployRef, error) {
	return nil, domain.ErrProductWorkflowRunNotFound
}

func cloneRun(run *model.ProductWorkflowRun) *model.ProductWorkflowRun {
	if run == nil {
		return nil
	}
	copyRun := *run
	copyRun.StageRuns = make([]model.StageRun, len(run.StageRuns))
	for i, stage := range run.StageRuns {
		copyRun.StageRuns[i] = stage
		copyRun.StageRuns[i].Jobs = append([]model.JobRun(nil), stage.Jobs...)
		for j, job := range stage.Jobs {
			copyRun.StageRuns[i].Jobs[j].Builds = append([]model.ProjectBuildRun(nil), job.Builds...)
		}
	}
	return &copyRun
}

func TestHandleBuildCompleted_concurrentUpdatesKeepBothBuildsSuccessful(t *testing.T) {
	buildA := kernel.NewBuildID()
	buildB := kernel.NewBuildID()
	runID := kernel.NewProductWorkflowRunID()
	wfID := kernel.NewProductWorkflowID()

	run := &model.ProductWorkflowRun{
		ID: runID, WorkflowID: wfID, Status: model.RunStatusRunning, CurrentStage: 0,
		StageRuns: []model.StageRun{{
			Name: "build", Status: model.JobRunStatusRunning,
			Jobs: []model.JobRun{{
				Name: "build-1", Type: model.JobTypeSparkopsBuild, Status: model.JobRunStatusRunning,
				Builds: []model.ProjectBuildRun{
					{BuildID: buildA.String(), Status: model.JobRunStatusRunning},
					{BuildID: buildB.String(), Status: model.JobRunStatusRunning},
				},
			}},
		}},
	}
	repo := &concurrentRunRepo{
		run: run,
		wf: &model.ProductWorkflow{
			ID: wfID,
			Stages: []model.WorkflowStage{
				{Name: "build", Jobs: []model.WorkflowJob{{Name: "build-1", Type: model.JobTypeSparkopsBuild}}},
			},
		},
		refs: map[string]*model.WorkflowBuildRef{
			buildA.String(): {BuildID: buildA, RunID: runID, JobName: "build-1"},
			buildB.String(): {BuildID: buildB, RunID: runID, JobName: "build-1"},
		},
	}
	scheduler := NewScheduler(repo, nil, nil, nil)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		require.NoError(t, scheduler.HandleBuildCompleted(context.Background(), buildA, string(buildmodel.BuildStatusSuccess)))
	}()
	go func() {
		defer wg.Done()
		require.NoError(t, scheduler.HandleBuildCompleted(context.Background(), buildB, string(buildmodel.BuildStatusSuccess)))
	}()
	wg.Wait()

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, model.JobRunStatusSuccess, repo.run.StageRuns[0].Jobs[0].Builds[0].Status)
	require.Equal(t, model.JobRunStatusSuccess, repo.run.StageRuns[0].Jobs[0].Builds[1].Status)
	require.Equal(t, model.RunStatusSuccess, repo.run.Status)
}

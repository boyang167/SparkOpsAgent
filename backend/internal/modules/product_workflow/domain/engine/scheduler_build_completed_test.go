package engine

import (
	"context"
	"sync"
	"testing"

	deployrunmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	buildmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type concurrentRunRepo struct {
	mu         sync.Mutex
	run        *model.ProductWorkflowRun
	wf         *model.ProductWorkflow
	refs       map[string]*model.WorkflowBuildRef
	deployRefs map[string]*model.WorkflowDeployRef
}

type conflictRunRepo struct {
	*concurrentRunRepo
	concurrentBuildID  string
	concurrentDeployID string
	alwaysConflict     bool
	updateAttempts     int
}

func (r *conflictRunRepo) UpdateRun(_ context.Context, run *model.ProductWorkflowRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updateAttempts++
	if r.alwaysConflict || r.updateAttempts == 1 {
		if !r.alwaysConflict {
			for si := range r.run.StageRuns {
				for ji := range r.run.StageRuns[si].Jobs {
					for bi := range r.run.StageRuns[si].Jobs[ji].Builds {
						build := &r.run.StageRuns[si].Jobs[ji].Builds[bi]
						if build.BuildID == r.concurrentBuildID {
							build.Status = model.JobRunStatusSuccess
						}
					}
					for di := range r.run.StageRuns[si].Jobs[ji].Deploys {
						deploy := &r.run.StageRuns[si].Jobs[ji].Deploys[di]
						if deploy.DeploymentRunID == r.concurrentDeployID {
							deploy.Status = model.JobRunStatusSuccess
						}
					}
				}
			}
			r.run.Version++
		}
		return domain.ErrProductWorkflowRunConflict
	}
	if run.Version != r.run.Version {
		return domain.ErrProductWorkflowRunConflict
	}
	saved := cloneRun(run)
	saved.Version++
	run.Version = saved.Version
	r.run = saved
	return nil
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
func (r *concurrentRunRepo) DeleteWorkflow(context.Context, kernel.ProductWorkflowID) error {
	return nil
}
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
func (r *concurrentRunRepo) SaveDeployRef(context.Context, *model.WorkflowDeployRef) error {
	return nil
}
func (r *concurrentRunRepo) FindRunByDeploymentID(_ context.Context, deploymentID kernel.DeploymentRunID) (*model.WorkflowDeployRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ref, ok := r.deployRefs[deploymentID.String()]
	if !ok {
		return nil, domain.ErrProductWorkflowRunNotFound
	}
	return ref, nil
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
	scheduler := NewScheduler(repo, nil, nil, nil, nil)

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

func TestHandleBuildCompletedReloadsAndMergesAfterVersionConflict(t *testing.T) {
	buildA := kernel.NewBuildID()
	buildB := kernel.NewBuildID()
	runID := kernel.NewProductWorkflowRunID()
	wfID := kernel.NewProductWorkflowID()
	base := &concurrentRunRepo{
		run: &model.ProductWorkflowRun{
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
		},
		wf: &model.ProductWorkflow{
			ID: wfID,
			Stages: []model.WorkflowStage{{
				Name: "build", Parallel: true,
				Jobs: []model.WorkflowJob{{Name: "build-1", Type: model.JobTypeSparkopsBuild}},
			}},
		},
		refs: map[string]*model.WorkflowBuildRef{
			buildA.String(): {BuildID: buildA, RunID: runID, JobName: "build-1"},
		},
	}
	repo := &conflictRunRepo{concurrentRunRepo: base, concurrentBuildID: buildB.String()}

	err := NewScheduler(repo, nil, nil, nil, nil).
		HandleBuildCompleted(context.Background(), buildA, string(buildmodel.BuildStatusSuccess))

	require.NoError(t, err)
	require.Equal(t, 4, repo.updateAttempts)
	require.Equal(t, model.JobRunStatusSuccess, repo.run.StageRuns[0].Jobs[0].Builds[0].Status)
	require.Equal(t, model.JobRunStatusSuccess, repo.run.StageRuns[0].Jobs[0].Builds[1].Status)
	require.Equal(t, model.RunStatusSuccess, repo.run.Status)
}

func TestHandleBuildCompletedStopsAfterBoundedVersionConflicts(t *testing.T) {
	buildID := kernel.NewBuildID()
	runID := kernel.NewProductWorkflowRunID()
	wfID := kernel.NewProductWorkflowID()
	base := &concurrentRunRepo{
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
		wf: &model.ProductWorkflow{
			ID: wfID,
			Stages: []model.WorkflowStage{{
				Name: "build", Jobs: []model.WorkflowJob{{Name: "build-1", Type: model.JobTypeSparkopsBuild}},
			}},
		},
		refs: map[string]*model.WorkflowBuildRef{
			buildID.String(): {BuildID: buildID, RunID: runID, JobName: "build-1"},
		},
	}
	repo := &conflictRunRepo{concurrentRunRepo: base, alwaysConflict: true}

	err := NewScheduler(repo, nil, nil, nil, nil).
		HandleBuildCompleted(context.Background(), buildID, string(buildmodel.BuildStatusSuccess))

	require.ErrorIs(t, err, domain.ErrProductWorkflowRunConflict)
	require.Equal(t, 3, repo.updateAttempts)
}

func TestHandleBuildCompletedDuplicateDoesNotRewriteStageRuns(t *testing.T) {
	buildA := kernel.NewBuildID()
	buildB := kernel.NewBuildID()
	runID := kernel.NewProductWorkflowRunID()
	wfID := kernel.NewProductWorkflowID()
	base := &concurrentRunRepo{
		run: &model.ProductWorkflowRun{
			ID: runID, WorkflowID: wfID, Status: model.RunStatusRunning,
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
		},
		wf: &model.ProductWorkflow{
			ID: wfID,
			Stages: []model.WorkflowStage{{
				Name: "build", Parallel: true,
				Jobs: []model.WorkflowJob{{Name: "build-1", Type: model.JobTypeSparkopsBuild}},
			}},
		},
		refs: map[string]*model.WorkflowBuildRef{
			buildA.String(): {BuildID: buildA, RunID: runID, JobName: "build-1"},
		},
	}
	repo := &conflictRunRepo{concurrentRunRepo: base, updateAttempts: 1}
	scheduler := NewScheduler(repo, nil, nil, nil, nil)

	require.NoError(t, scheduler.HandleBuildCompleted(
		context.Background(), buildA, string(buildmodel.BuildStatusSuccess),
	))
	require.NoError(t, scheduler.HandleBuildCompleted(
		context.Background(), buildA, string(buildmodel.BuildStatusSuccess),
	))

	require.Equal(t, 2, repo.updateAttempts)
	require.Equal(t, int64(1), repo.run.Version)
	require.Equal(t, model.JobRunStatusSuccess, repo.run.StageRuns[0].Jobs[0].Builds[0].Status)
	require.Equal(t, model.JobRunStatusRunning, repo.run.StageRuns[0].Jobs[0].Builds[1].Status)
}

func TestHandleDeployCompletedReloadsAndMergesAfterVersionConflict(t *testing.T) {
	deployA := kernel.NewDeploymentRunID()
	deployB := kernel.NewDeploymentRunID()
	runID := kernel.NewProductWorkflowRunID()
	wfID := kernel.NewProductWorkflowID()
	base := &concurrentRunRepo{
		run: &model.ProductWorkflowRun{
			ID: runID, WorkflowID: wfID, Status: model.RunStatusRunning, CurrentStage: 0,
			StageRuns: []model.StageRun{{
				Name: "deploy", Status: model.JobRunStatusRunning,
				Jobs: []model.JobRun{{
					Name: "deploy-1", Type: model.JobTypeSparkopsDeploy, Status: model.JobRunStatusRunning,
					Deploys: []model.ProjectDeployRun{
						{DeploymentRunID: deployA.String(), Status: model.JobRunStatusRunning},
						{DeploymentRunID: deployB.String(), Status: model.JobRunStatusRunning},
					},
				}},
			}},
		},
		wf: &model.ProductWorkflow{
			ID: wfID,
			Stages: []model.WorkflowStage{{
				Name: "deploy", Jobs: []model.WorkflowJob{{Name: "deploy-1", Type: model.JobTypeSparkopsDeploy}},
			}},
		},
		deployRefs: map[string]*model.WorkflowDeployRef{
			deployA.String(): {DeploymentRunID: deployA, RunID: runID, JobName: "deploy-1"},
		},
	}
	repo := &conflictRunRepo{concurrentRunRepo: base, concurrentDeployID: deployB.String()}

	err := NewScheduler(repo, nil, nil, nil, nil).
		HandleDeployCompleted(context.Background(), deployA, string(deployrunmodel.StatusSucceeded))

	require.NoError(t, err)
	require.Equal(t, model.JobRunStatusSuccess, repo.run.StageRuns[0].Jobs[0].Deploys[0].Status)
	require.Equal(t, model.JobRunStatusSuccess, repo.run.StageRuns[0].Jobs[0].Deploys[1].Status)
	require.Equal(t, model.RunStatusSuccess, repo.run.Status)
}

func TestHandleBuildCompleted_partialStageRunCompletes(t *testing.T) {
	buildA := kernel.NewBuildID()
	buildB := kernel.NewBuildID()
	runID := kernel.NewProductWorkflowRunID()
	wfID := kernel.NewProductWorkflowID()

	run := &model.ProductWorkflowRun{
		ID: runID, WorkflowID: wfID, Status: model.RunStatusRunning, CurrentStage: 0,
		Input: []byte(`{"stage_indices":[0]}`),
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
				{Name: "deploy", Jobs: []model.WorkflowJob{{Name: "deploy-1", Type: model.JobTypeSparkopsDeploy}}},
			},
		},
		refs: map[string]*model.WorkflowBuildRef{
			buildA.String(): {BuildID: buildA, RunID: runID, JobName: "build-1"},
			buildB.String(): {BuildID: buildB, RunID: runID, JobName: "build-1"},
		},
	}
	scheduler := NewScheduler(repo, nil, nil, nil, nil)

	require.NoError(t, scheduler.HandleBuildCompleted(context.Background(), buildA, string(buildmodel.BuildStatusSuccess)))
	require.NoError(t, scheduler.HandleBuildCompleted(context.Background(), buildB, string(buildmodel.BuildStatusSuccess)))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, model.RunStatusSuccess, repo.run.Status)
	require.NotNil(t, repo.run.CompletedAt)
}

func TestHealStuckRun_completesWhenAllStagesDone(t *testing.T) {
	runID := kernel.NewProductWorkflowRunID()
	wfID := kernel.NewProductWorkflowID()
	run := &model.ProductWorkflowRun{
		ID: runID, WorkflowID: wfID, Status: model.RunStatusRunning, CurrentStage: 1,
		Input: []byte(`{"stage_indices":[0]}`),
		StageRuns: []model.StageRun{{
			Name: "build", Status: model.JobRunStatusSuccess,
			Jobs: []model.JobRun{{
				Name: "build-1", Type: model.JobTypeSparkopsBuild, Status: model.JobRunStatusSuccess,
			}},
		}},
	}
	repo := &concurrentRunRepo{
		run: run,
		wf: &model.ProductWorkflow{
			ID: wfID,
			Stages: []model.WorkflowStage{
				{Name: "build", Jobs: []model.WorkflowJob{{Name: "build-1", Type: model.JobTypeSparkopsBuild}}},
				{Name: "deploy", Jobs: []model.WorkflowJob{{Name: "deploy-1", Type: model.JobTypeSparkopsDeploy}}},
			},
		},
	}
	scheduler := NewScheduler(repo, nil, nil, nil, nil)
	require.NoError(t, scheduler.healStuckRun(context.Background(), run))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, model.RunStatusSuccess, repo.run.Status)
}

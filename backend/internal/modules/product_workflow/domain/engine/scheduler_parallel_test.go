package engine

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type updateOnlyRepo struct {
	run *model.ProductWorkflowRun
}

func (r *updateOnlyRepo) UpdateRun(_ context.Context, run *model.ProductWorkflowRun) error {
	r.run = run
	return nil
}

func (r *updateOnlyRepo) SaveWorkflow(context.Context, *model.ProductWorkflow) error { return nil }
func (r *updateOnlyRepo) UpdateWorkflow(context.Context, *model.ProductWorkflow) error {
	return nil
}
func (r *updateOnlyRepo) DeleteWorkflow(context.Context, kernel.ProductWorkflowID) error { return nil }
func (r *updateOnlyRepo) FindWorkflowByID(context.Context, kernel.ProductWorkflowID) (*model.ProductWorkflow, error) {
	return nil, nil
}
func (r *updateOnlyRepo) ListWorkflowsByProduct(context.Context, kernel.ProductID) ([]*model.ProductWorkflow, error) {
	return nil, nil
}
func (r *updateOnlyRepo) ListWorkflowsByWorkspace(context.Context, kernel.WorkspaceID) ([]*model.ProductWorkflow, error) {
	return nil, nil
}
func (r *updateOnlyRepo) ListAllWorkflows(context.Context) ([]*model.ProductWorkflow, error) {
	return nil, nil
}
func (r *updateOnlyRepo) CountActiveRuns(context.Context, kernel.ProductWorkflowID) (int64, error) {
	return 0, nil
}
func (r *updateOnlyRepo) ListActiveRuns(context.Context, int) ([]*model.ProductWorkflowRun, error) {
	return nil, nil
}
func (r *updateOnlyRepo) SaveRun(context.Context, *model.ProductWorkflowRun) error { return nil }
func (r *updateOnlyRepo) FindRunByID(context.Context, kernel.ProductWorkflowRunID) (*model.ProductWorkflowRun, error) {
	return nil, nil
}
func (r *updateOnlyRepo) ListRunsByWorkflow(context.Context, kernel.ProductWorkflowID, int) ([]*model.ProductWorkflowRun, error) {
	return nil, nil
}
func (r *updateOnlyRepo) SaveBuildRef(context.Context, *model.WorkflowBuildRef) error { return nil }
func (r *updateOnlyRepo) FindRunByBuildID(context.Context, kernel.BuildID) (*model.WorkflowBuildRef, error) {
	return nil, nil
}
func (r *updateOnlyRepo) SaveDeployRef(context.Context, *model.WorkflowDeployRef) error { return nil }
func (r *updateOnlyRepo) FindRunByDeploymentID(context.Context, kernel.DeploymentRunID) (*model.WorkflowDeployRef, error) {
	return nil, nil
}

var _ repository.ProductWorkflowRepository = (*updateOnlyRepo)(nil)

func TestExecuteCurrentStage_sequentialStartsOnlyFirstApproval(t *testing.T) {
	stages := []model.WorkflowStage{{
		Name:     "审批",
		Parallel: false,
		Jobs: []model.WorkflowJob{
			{Name: "a1", Type: model.JobTypeSparkopsApproval},
			{Name: "a2", Type: model.JobTypeSparkopsApproval},
		},
	}}
	wf := &model.ProductWorkflow{Stages: stages}
	run := model.NewProductWorkflowRun(kernel.NewProductWorkflowID(), kernel.ProductID{}, stages)
	repo := &updateOnlyRepo{}
	err := NewScheduler(repo, nil, nil, nil).StartRun(context.Background(), wf, run)
	require.NoError(t, err)
	require.Equal(t, model.JobRunStatusRunning, run.StageRuns[0].Jobs[0].Status)
	require.Equal(t, model.JobRunStatusPending, run.StageRuns[0].Jobs[1].Status)
}

func TestExecuteCurrentStage_parallelStartsEveryApproval(t *testing.T) {
	stages := []model.WorkflowStage{{
		Name:     "审批",
		Parallel: true,
		Jobs: []model.WorkflowJob{
			{Name: "a1", Type: model.JobTypeSparkopsApproval},
			{Name: "a2", Type: model.JobTypeSparkopsApproval},
		},
	}}
	wf := &model.ProductWorkflow{Stages: stages}
	run := model.NewProductWorkflowRun(kernel.NewProductWorkflowID(), kernel.ProductID{}, stages)
	err := NewScheduler(&updateOnlyRepo{}, nil, nil, nil).StartRun(context.Background(), wf, run)
	require.NoError(t, err)
	require.Equal(t, model.JobRunStatusRunning, run.StageRuns[0].Jobs[0].Status)
	require.Equal(t, model.JobRunStatusRunning, run.StageRuns[0].Jobs[1].Status)
}

func TestExecuteCurrentStage_debugPausesBeforeJobs(t *testing.T) {
	stages := []model.WorkflowStage{{
		Name: "审批",
		Jobs: []model.WorkflowJob{{Name: "a1", Type: model.JobTypeSparkopsApproval}},
	}}
	wf := &model.ProductWorkflow{Stages: stages, Advanced: model.WorkflowAdvanced{Debug: true}}
	run := model.NewProductWorkflowRun(kernel.NewProductWorkflowID(), kernel.ProductID{}, stages)
	err := NewScheduler(&updateOnlyRepo{}, nil, nil, nil).StartRun(context.Background(), wf, run)
	require.NoError(t, err)
	require.Equal(t, model.RunStatusPaused, run.Status)
	require.Equal(t, model.JobRunStatusPending, run.StageRuns[0].Jobs[0].Status)
}

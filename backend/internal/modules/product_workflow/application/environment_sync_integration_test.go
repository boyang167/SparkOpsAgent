package application_test

import (
	"context"
	"encoding/json"
	"testing"

	deploydomain "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	deploymodel "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	productsubscriber "github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/subscriber"
	productdomain "github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	productmodel "github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	pwfdomain "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type integrationEnvProjectRepo struct {
	rows  map[string]*productmodel.EnvironmentProject
	saved map[string]*productmodel.EnvironmentProject
}

func envProjectKey(envID kernel.EnvironmentID, projectID kernel.ProjectID) string {
	return envID.String() + ":" + projectID.String()
}

func (r *integrationEnvProjectRepo) FindEnvironmentProject(_ context.Context, envID kernel.EnvironmentID, projectID kernel.ProjectID) (*productmodel.EnvironmentProject, error) {
	if r.rows == nil {
		return nil, productdomain.ErrEnvironmentProjectNotFound
	}
	row, ok := r.rows[envProjectKey(envID, projectID)]
	if !ok || row == nil {
		return nil, productdomain.ErrEnvironmentProjectNotFound
	}
	return row, nil
}

func (r *integrationEnvProjectRepo) SaveEnvironmentProject(_ context.Context, row *productmodel.EnvironmentProject) error {
	if r.saved == nil {
		r.saved = map[string]*productmodel.EnvironmentProject{}
	}
	r.saved[envProjectKey(row.EnvironmentID, row.ProjectID)] = row
	return nil
}

func (integrationEnvProjectRepo) SaveEnvironment(context.Context, *productmodel.Environment) error {
	return nil
}
func (integrationEnvProjectRepo) FindEnvironmentByID(context.Context, kernel.EnvironmentID) (*productmodel.Environment, error) {
	return nil, productdomain.ErrEnvironmentProjectNotFound
}
func (integrationEnvProjectRepo) ListEnvironmentsByProduct(context.Context, kernel.ProductID) ([]*productmodel.Environment, error) {
	return nil, nil
}
func (integrationEnvProjectRepo) DeleteEnvironment(context.Context, kernel.EnvironmentID) error { return nil }
func (integrationEnvProjectRepo) CountEnvironmentsByProduct(context.Context, kernel.ProductID) (int, error) {
	return 0, nil
}
func (integrationEnvProjectRepo) ListEnvironmentProjects(context.Context, kernel.EnvironmentID) ([]*productmodel.EnvironmentProject, error) {
	return nil, nil
}
func (integrationEnvProjectRepo) EnsureEnvironmentProjects(context.Context, kernel.EnvironmentID, []kernel.ProjectID) error {
	return nil
}

type integrationEnvSyncSnapshots struct {
	snapshot *appctx.EnvironmentSyncSnapshot
}

func (s integrationEnvSyncSnapshots) Validate(context.Context, kernel.EnvironmentSyncSnapshotID, kernel.ProductID, kernel.ProductWorkflowID, string) error {
	return nil
}

func (s integrationEnvSyncSnapshots) BindMany(context.Context, []appctx.EnvironmentSyncSnapshotBinding) error {
	return nil
}

func (s integrationEnvSyncSnapshots) LoadPreview(context.Context, kernel.EnvironmentSyncSnapshotID) (*appctx.EnvironmentSyncSnapshot, error) {
	return s.snapshot, nil
}

func (s integrationEnvSyncSnapshots) LoadBound(context.Context, kernel.EnvironmentSyncSnapshotID, kernel.ProductWorkflowRunID) (*appctx.EnvironmentSyncSnapshot, error) {
	return s.snapshot, nil
}

type integrationEnvSyncTrigger struct {
	failProject kernel.ProjectID
	attempts    []kernel.ProjectID
	deployIDs   map[string]kernel.DeploymentRunID
}

func (t *integrationEnvSyncTrigger) TriggerEnvironmentSync(_ context.Context, _ kernel.EnvironmentSyncSnapshotID, projectID kernel.ProjectID) (kernel.DeploymentRunID, error) {
	t.attempts = append(t.attempts, projectID)
	if projectID == t.failProject {
		return kernel.DeploymentRunID{}, deploydomain.ErrDeploymentAlreadyRunning
	}
	if t.deployIDs == nil {
		t.deployIDs = map[string]kernel.DeploymentRunID{}
	}
	deployID := kernel.NewDeploymentRunID()
	t.deployIDs[projectID.String()] = deployID
	return deployID, nil
}

type integrationWorkflowRepo struct {
	run        *model.ProductWorkflowRun
	wf         *model.ProductWorkflow
	deployRefs map[string]*model.WorkflowDeployRef
}

func (r *integrationWorkflowRepo) UpdateRun(_ context.Context, run *model.ProductWorkflowRun) error {
	r.run = cloneIntegrationRun(run)
	return nil
}

func (r *integrationWorkflowRepo) FindRunByID(context.Context, kernel.ProductWorkflowRunID) (*model.ProductWorkflowRun, error) {
	return cloneIntegrationRun(r.run), nil
}

func (r *integrationWorkflowRepo) FindWorkflowByID(context.Context, kernel.ProductWorkflowID) (*model.ProductWorkflow, error) {
	return r.wf, nil
}

func (r *integrationWorkflowRepo) SaveDeployRef(_ context.Context, ref *model.WorkflowDeployRef) error {
	if r.deployRefs == nil {
		r.deployRefs = map[string]*model.WorkflowDeployRef{}
	}
	r.deployRefs[ref.DeploymentRunID.String()] = ref
	return nil
}

func (r *integrationWorkflowRepo) FindRunByDeploymentID(_ context.Context, deploymentID kernel.DeploymentRunID) (*model.WorkflowDeployRef, error) {
	ref, ok := r.deployRefs[deploymentID.String()]
	if !ok {
		return nil, pwfdomain.ErrProductWorkflowRunNotFound
	}
	return ref, nil
}

func cloneIntegrationRun(run *model.ProductWorkflowRun) *model.ProductWorkflowRun {
	if run == nil {
		return nil
	}
	cloned := *run
	cloned.StageRuns = make([]model.StageRun, len(run.StageRuns))
	for si, stage := range run.StageRuns {
		clonedStage := stage
		clonedStage.Jobs = make([]model.JobRun, len(stage.Jobs))
		for ji, job := range stage.Jobs {
			clonedJob := job
			clonedJob.Deploys = append([]model.ProjectDeployRun(nil), job.Deploys...)
			clonedStage.Jobs[ji] = clonedJob
		}
		cloned.StageRuns[si] = clonedStage
	}
	return &cloned
}

func (integrationWorkflowRepo) SaveWorkflow(context.Context, *model.ProductWorkflow) error { return nil }
func (integrationWorkflowRepo) UpdateWorkflow(context.Context, *model.ProductWorkflow) error {
	return nil
}
func (integrationWorkflowRepo) DeleteWorkflow(context.Context, kernel.ProductWorkflowID) error { return nil }
func (integrationWorkflowRepo) ListWorkflowsByProduct(context.Context, kernel.ProductID) ([]*model.ProductWorkflow, error) {
	return nil, nil
}
func (integrationWorkflowRepo) ListWorkflowsByWorkspace(context.Context, kernel.WorkspaceID) ([]*model.ProductWorkflow, error) {
	return nil, nil
}
func (integrationWorkflowRepo) ListAllWorkflows(context.Context) ([]*model.ProductWorkflow, error) {
	return nil, nil
}
func (integrationWorkflowRepo) CountActiveRuns(context.Context, kernel.ProductWorkflowID) (int64, error) {
	return 0, nil
}
func (integrationWorkflowRepo) ListActiveRuns(context.Context, int) ([]*model.ProductWorkflowRun, error) {
	return nil, nil
}
func (integrationWorkflowRepo) SaveRun(context.Context, *model.ProductWorkflowRun) error { return nil }
func (integrationWorkflowRepo) ListRunsByWorkflow(context.Context, kernel.ProductWorkflowID, int) ([]*model.ProductWorkflowRun, error) {
	return nil, nil
}
func (integrationWorkflowRepo) CountRunsByWorkflow(context.Context, kernel.ProductWorkflowID) (int64, error) {
	return 0, nil
}
func (integrationWorkflowRepo) SaveBuildRef(context.Context, *model.WorkflowBuildRef) error { return nil }
func (integrationWorkflowRepo) FindRunByBuildID(context.Context, kernel.BuildID) (*model.WorkflowBuildRef, error) {
	return nil, nil
}

func TestEnvironmentSyncWorkflowContinuesProjectsAndAssociatesOnlySuccess(t *testing.T) {
	ctx := context.Background()
	sourceEnv := kernel.NewEnvironmentID()
	targetEnv := kernel.NewEnvironmentID()
	projectOK := kernel.NewProjectID()
	projectFail := kernel.NewProjectID()
	snapshotID := kernel.NewEnvironmentSyncSnapshotID()
	runID := kernel.NewProductWorkflowRunID()
	wfID := kernel.NewProductWorkflowID()
	input, err := json.Marshal(map[string]any{
		"environment_sync_preview_ids": map[string]any{
			"promote": snapshotID.String(),
		},
	})
	require.NoError(t, err)

	run := &model.ProductWorkflowRun{
		ID: runID, WorkflowID: wfID, Status: model.RunStatusRunning, CurrentStage: 0,
		Input: input,
		StageRuns: []model.StageRun{{
			Name: "sync", Status: model.JobRunStatusPending,
			Jobs: []model.JobRun{{Name: "promote", Status: model.JobRunStatusPending}},
		}},
	}
	wf := &model.ProductWorkflow{
		ID: wfID,
		Stages: []model.WorkflowStage{{
			Name: "sync",
			Jobs: []model.WorkflowJob{{Name: "promote", Type: model.JobTypeSparkopsEnvironmentSync}},
		}},
	}
	repo := &integrationWorkflowRepo{run: run, wf: wf}
	trigger := &integrationEnvSyncTrigger{failProject: projectFail}
	snapshots := integrationEnvSyncSnapshots{
		snapshot: &appctx.EnvironmentSyncSnapshot{
			ID: snapshotID, SourceEnvironmentID: sourceEnv, TargetEnvironmentID: targetEnv,
			Projects: []appctx.EnvironmentSyncProjectSnapshot{
				{
					ProjectID: projectOK,
					Option:    appctx.EnvironmentSyncProjectOption{ProjectID: projectOK, SyncImage: true, FirstSync: true},
				},
				{
					ProjectID: projectFail,
					Option:    appctx.EnvironmentSyncProjectOption{ProjectID: projectFail, SyncImage: true, SyncYAML: true},
				},
			},
		},
	}

	executor := engine.NewEnvironmentSyncExecutor(snapshots, trigger, repo)
	require.NoError(t, executor.RunJob(ctx, run, 0, 0, wf.Stages[0].Jobs[0]))

	jobRun := run.StageRuns[0].Jobs[0]
	require.Len(t, trigger.attempts, 2)
	require.Len(t, jobRun.Deploys, 2)
	require.Equal(t, model.JobRunStatusRunning, jobRun.Deploys[0].Status)
	require.Equal(t, model.JobRunStatusFailed, jobRun.Deploys[1].Status)
	require.Equal(t, model.JobRunStatusRunning, jobRun.Status)

	deployOK, ok := trigger.deployIDs[projectOK.String()]
	require.True(t, ok)
	require.NotEmpty(t, deployOK.String())

	envRepo := &integrationEnvProjectRepo{rows: map[string]*productmodel.EnvironmentProject{}}
	productHandler := productsubscriber.NewOnDeployCompleted(envRepo)
	ev := deliveryevent.NewDeployCompleted(
		deployOK, projectOK, kernel.NewWorkspaceID(), &targetEnv, "", "succeeded",
		string(deploymodel.RunTypeEnvironmentSync), "v1.2.3",
	)
	ev.SourceEnvironmentID = &sourceEnv
	ev.EnvironmentSyncSnapshotID = &snapshotID
	require.NoError(t, productHandler.Handle(ctx, ev))

	okKey := envProjectKey(targetEnv, projectOK)
	require.NotNil(t, envRepo.saved[okKey])
	require.True(t, envRepo.saved[okKey].Enabled)
	require.Equal(t, "v1.2.3", envRepo.saved[okKey].CurrentImageTag)
	require.NotNil(t, envRepo.saved[okKey].CurrentDeploymentRunID)
	require.Equal(t, deployOK, *envRepo.saved[okKey].CurrentDeploymentRunID)

	failKey := envProjectKey(targetEnv, projectFail)
	require.Nil(t, envRepo.saved[failKey])

	scheduler := engine.NewScheduler(repo, nil, nil, nil, engine.NewEnvironmentSyncExecutor(snapshots, trigger, repo), nil)
	require.NoError(t, scheduler.HandleDeployCompleted(ctx, deployOK, string(deploymodel.StatusSucceeded)))

	require.Equal(t, model.JobRunStatusSuccess, repo.run.StageRuns[0].Jobs[0].Deploys[0].Status)
	require.Equal(t, model.JobRunStatusFailed, repo.run.StageRuns[0].Jobs[0].Deploys[1].Status)
	require.Equal(t, model.JobRunStatusFailed, repo.run.StageRuns[0].Jobs[0].Status)
	require.Equal(t, model.RunStatusFailed, repo.run.Status)
}

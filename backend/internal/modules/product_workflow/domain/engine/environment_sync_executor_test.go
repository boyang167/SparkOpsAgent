package engine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	deploydomain "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type stubEnvironmentSyncSnapshots struct {
	snapshot *appctx.EnvironmentSyncSnapshot
}

func (s stubEnvironmentSyncSnapshots) Validate(context.Context, kernel.EnvironmentSyncSnapshotID, kernel.ProductID, kernel.ProductWorkflowID, string) error {
	return nil
}

func (s stubEnvironmentSyncSnapshots) BindMany(context.Context, []appctx.EnvironmentSyncSnapshotBinding) error {
	return nil
}

func (s stubEnvironmentSyncSnapshots) LoadPreview(context.Context, kernel.EnvironmentSyncSnapshotID) (*appctx.EnvironmentSyncSnapshot, error) {
	return s.snapshot, nil
}

func (s stubEnvironmentSyncSnapshots) LoadBound(_ context.Context, _ kernel.EnvironmentSyncSnapshotID, _ kernel.ProductWorkflowRunID) (*appctx.EnvironmentSyncSnapshot, error) {
	return s.snapshot, nil
}

type stubEnvironmentSyncTrigger struct {
	failProject kernel.ProjectID
}

func (s stubEnvironmentSyncTrigger) TriggerEnvironmentSync(_ context.Context, _ kernel.EnvironmentSyncSnapshotID, projectID kernel.ProjectID) (kernel.DeploymentRunID, error) {
	if projectID == s.failProject {
		return kernel.DeploymentRunID{}, deploydomain.ErrDeploymentAlreadyRunning
	}
	return kernel.NewDeploymentRunID(), nil
}

func TestEnvironmentSyncExecutorContinuesAfterProjectFailure(t *testing.T) {
	p1 := kernel.NewProjectID()
	p2 := kernel.NewProjectID()
	p3 := kernel.NewProjectID()
	snapshotID := kernel.NewEnvironmentSyncSnapshotID()
	runID := kernel.NewProductWorkflowRunID()
	sourceEnv := kernel.NewEnvironmentID()
	targetEnv := kernel.NewEnvironmentID()
	input, err := json.Marshal(map[string]any{
		"environment_sync_preview_ids": map[string]any{
			"sync-1": snapshotID.String(),
		},
	})
	require.NoError(t, err)

	run := &model.ProductWorkflowRun{
		ID: runID, Status: model.RunStatusRunning,
		Input: input,
		StageRuns: []model.StageRun{{
			Name: "sync", Status: model.JobRunStatusPending,
			Jobs: []model.JobRun{{Name: "sync-1", Status: model.JobRunStatusPending}},
		}},
	}
	job := model.WorkflowJob{
		Name: "sync-1", Type: model.JobTypeSparkopsEnvironmentSync,
		Spec: map[string]any{
			"source_environment_id": sourceEnv.String(),
			"target_environment_id": targetEnv.String(),
		},
	}
	repo := &concurrentRunRepo{run: run}
	executor := NewEnvironmentSyncExecutor(stubEnvironmentSyncSnapshots{
		snapshot: &appctx.EnvironmentSyncSnapshot{
			ID: snapshotID, SourceEnvironmentID: sourceEnv, TargetEnvironmentID: targetEnv,
			Projects: []appctx.EnvironmentSyncProjectSnapshot{
				{ProjectID: p1, Option: appctx.EnvironmentSyncProjectOption{ProjectID: p1, SyncImage: true}},
				{ProjectID: p2, Option: appctx.EnvironmentSyncProjectOption{ProjectID: p2, SyncImage: true, SyncYAML: true}},
				{ProjectID: p3, Option: appctx.EnvironmentSyncProjectOption{ProjectID: p3, SyncImage: true, FirstSync: true}},
			},
		},
	}, stubEnvironmentSyncTrigger{failProject: p2}, repo)

	require.NoError(t, executor.RunJob(context.Background(), run, 0, 0, job))

	jobRun := run.StageRuns[0].Jobs[0]
	require.Len(t, jobRun.Deploys, 3)
	require.Equal(t, model.JobRunStatusRunning, jobRun.Deploys[0].Status)
	require.Equal(t, model.JobRunStatusFailed, jobRun.Deploys[1].Status)
	require.Equal(t, model.JobRunStatusRunning, jobRun.Deploys[2].Status)
	require.Equal(t, model.JobRunStatusRunning, jobRun.Status)
	require.True(t, jobRun.Deploys[1].SyncYAML)
	require.True(t, jobRun.Deploys[2].FirstSync)
	require.Equal(t, sourceEnv.String(), jobRun.Deploys[0].SourceEnvironmentID)
}

func TestEnvironmentSyncJobFailsAfterAllProjectsFinishWhenAnyFailed(t *testing.T) {
	jobRun := &model.JobRun{
		Name: "sync-1", Type: model.JobTypeSparkopsEnvironmentSync, Status: model.JobRunStatusRunning,
		Deploys: []model.ProjectDeployRun{
			{ProjectID: "p1", Status: model.JobRunStatusSuccess},
			{ProjectID: "p2", Status: model.JobRunStatusFailed, ErrorMessage: "conflict"},
			{ProjectID: "p3", Status: model.JobRunStatusSuccess},
		},
	}
	failed, msg := refreshEnvironmentSyncDeployJob(jobRun)
	require.True(t, failed)
	require.Equal(t, "environment sync job failed", msg)
	require.Equal(t, model.JobRunStatusFailed, jobRun.Status)
}

func TestEnvironmentSyncTriggerUnavailable(t *testing.T) {
	run := &model.ProductWorkflowRun{
		ID: kernel.NewProductWorkflowRunID(),
		StageRuns: []model.StageRun{{
			Jobs: []model.JobRun{{Name: "sync-1"}},
		}},
	}
	executor := NewEnvironmentSyncExecutor(stubEnvironmentSyncSnapshots{}, nil, &concurrentRunRepo{run: run})
	err := executor.RunJob(context.Background(), run, 0, 0, model.WorkflowJob{
		Name: "sync-1", Type: model.JobTypeSparkopsEnvironmentSync,
	})
	require.Error(t, err)
	require.Equal(t, model.JobRunStatusFailed, run.StageRuns[0].Jobs[0].Status)
}

func TestEnvironmentSyncExecutorPropagatesSaveDeployRefError(t *testing.T) {
	p1 := kernel.NewProjectID()
	snapshotID := kernel.NewEnvironmentSyncSnapshotID()
	runID := kernel.NewProductWorkflowRunID()
	input, err := json.Marshal(map[string]any{
		"environment_sync_preview_ids": map[string]any{"sync-1": snapshotID.String()},
	})
	require.NoError(t, err)
	run := &model.ProductWorkflowRun{
		ID: runID, Input: input,
		StageRuns: []model.StageRun{{Jobs: []model.JobRun{{Name: "sync-1"}}}},
	}
	repo := &saveDeployRefErrorRepo{}
	executor := NewEnvironmentSyncExecutor(stubEnvironmentSyncSnapshots{
		snapshot: &appctx.EnvironmentSyncSnapshot{
			ID: snapshotID,
			Projects: []appctx.EnvironmentSyncProjectSnapshot{
				{ProjectID: p1, Option: appctx.EnvironmentSyncProjectOption{ProjectID: p1, SyncImage: true}},
			},
		},
	}, stubEnvironmentSyncTrigger{}, repo)
	err = executor.RunJob(context.Background(), run, 0, 0, model.WorkflowJob{
		Name: "sync-1", Type: model.JobTypeSparkopsEnvironmentSync,
	})
	require.Error(t, err)
}

type saveDeployRefErrorRepo struct {
	concurrentRunRepo
}

func (saveDeployRefErrorRepo) SaveDeployRef(context.Context, *model.WorkflowDeployRef) error {
	return errors.New("save deploy ref failed")
}

package command

import (
	"context"
	"testing"

	deployrun "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type memoryEnvironmentSyncSnapshots struct {
	byID map[string]*appctx.EnvironmentSyncSnapshot
}

func (m *memoryEnvironmentSyncSnapshots) Validate(
	_ context.Context,
	snapshotID kernel.EnvironmentSyncSnapshotID,
	productID kernel.ProductID,
	workflowID kernel.ProductWorkflowID,
	jobName string,
) error {
	snapshot, ok := m.byID[snapshotID.String()]
	if !ok {
		return deployrun.ErrEnvironmentSyncSnapshotNotFound
	}
	if snapshot.ProductID != productID || snapshot.WorkflowID != workflowID || snapshot.JobName != jobName {
		return deployrun.ErrEnvironmentSyncSnapshotNotFound
	}
	if snapshot.BoundRunID != nil {
		return deployrun.ErrEnvironmentSyncSnapshotAlreadyBound
	}
	return nil
}

func (m *memoryEnvironmentSyncSnapshots) BindMany(_ context.Context, bindings []appctx.EnvironmentSyncSnapshotBinding) error {
	for _, binding := range bindings {
		snapshot := m.byID[binding.SnapshotID.String()]
		if snapshot == nil {
			return deployrun.ErrEnvironmentSyncSnapshotNotFound
		}
		runID := binding.RunID
		snapshot.BoundRunID = &runID
	}
	return nil
}

func (m *memoryEnvironmentSyncSnapshots) LoadPreview(_ context.Context, snapshotID kernel.EnvironmentSyncSnapshotID) (*appctx.EnvironmentSyncSnapshot, error) {
	snapshot, ok := m.byID[snapshotID.String()]
	if !ok {
		return nil, deployrun.ErrEnvironmentSyncSnapshotNotFound
	}
	if snapshot.BoundRunID != nil {
		return nil, deployrun.ErrEnvironmentSyncSnapshotAlreadyBound
	}
	return snapshot, nil
}

func (m *memoryEnvironmentSyncSnapshots) LoadBound(_ context.Context, snapshotID kernel.EnvironmentSyncSnapshotID, runID kernel.ProductWorkflowRunID) (*appctx.EnvironmentSyncSnapshot, error) {
	snapshot, ok := m.byID[snapshotID.String()]
	if !ok {
		return nil, deployrun.ErrEnvironmentSyncSnapshotNotFound
	}
	if snapshot.BoundRunID == nil || *snapshot.BoundRunID != runID {
		return nil, deployrun.ErrEnvironmentSyncSnapshotNotBound
	}
	return snapshot, nil
}

type stubProductChecker struct {
	wsID kernel.WorkspaceID
}

func (s stubProductChecker) ProductWorkspace(context.Context, kernel.ProductID) (kernel.WorkspaceID, error) {
	return s.wsID, nil
}

func (stubProductChecker) ProductName(context.Context, kernel.ProductID) (string, error) {
	return "demo", nil
}

type startRunWorkflowRepo struct {
	saved *model.ProductWorkflowRun
}

func (r *startRunWorkflowRepo) SaveRun(_ context.Context, run *model.ProductWorkflowRun) error {
	r.saved = run
	return nil
}

func (startRunWorkflowRepo) FindRunByID(context.Context, kernel.ProductWorkflowRunID) (*model.ProductWorkflowRun, error) {
	return nil, domain.ErrProductWorkflowRunNotFound
}
func (startRunWorkflowRepo) FindWorkflowByID(context.Context, kernel.ProductWorkflowID) (*model.ProductWorkflow, error) {
	return nil, domain.ErrProductWorkflowNotFound
}
func (startRunWorkflowRepo) SaveWorkflow(context.Context, *model.ProductWorkflow) error { return nil }
func (startRunWorkflowRepo) UpdateWorkflow(context.Context, *model.ProductWorkflow) error {
	return nil
}
func (startRunWorkflowRepo) DeleteWorkflow(context.Context, kernel.ProductWorkflowID) error { return nil }
func (startRunWorkflowRepo) ListWorkflowsByProduct(context.Context, kernel.ProductID) ([]*model.ProductWorkflow, error) {
	return nil, nil
}
func (startRunWorkflowRepo) ListWorkflowsByWorkspace(context.Context, kernel.WorkspaceID) ([]*model.ProductWorkflow, error) {
	return nil, nil
}
func (startRunWorkflowRepo) ListAllWorkflows(context.Context) ([]*model.ProductWorkflow, error) {
	return nil, nil
}
func (startRunWorkflowRepo) CountActiveRuns(context.Context, kernel.ProductWorkflowID) (int64, error) {
	return 0, nil
}
func (startRunWorkflowRepo) ListActiveRuns(context.Context, int) ([]*model.ProductWorkflowRun, error) {
	return nil, nil
}
func (startRunWorkflowRepo) UpdateRun(context.Context, *model.ProductWorkflowRun) error { return nil }
func (startRunWorkflowRepo) ListRunsByWorkflow(context.Context, kernel.ProductWorkflowID, int) ([]*model.ProductWorkflowRun, error) {
	return nil, nil
}
func (startRunWorkflowRepo) CountRunsByWorkflow(context.Context, kernel.ProductWorkflowID) (int64, error) {
	return 0, nil
}
func (startRunWorkflowRepo) SaveBuildRef(context.Context, *model.WorkflowBuildRef) error { return nil }
func (startRunWorkflowRepo) SaveDeployRef(context.Context, *model.WorkflowDeployRef) error {
	return nil
}
func (startRunWorkflowRepo) FindRunByBuildID(context.Context, kernel.BuildID) (*model.WorkflowBuildRef, error) {
	return nil, domain.ErrProductWorkflowRunNotFound
}
func (startRunWorkflowRepo) FindRunByDeploymentID(context.Context, kernel.DeploymentRunID) (*model.WorkflowDeployRef, error) {
	return nil, domain.ErrProductWorkflowRunNotFound
}

func environmentSyncWorkflow(productID kernel.ProductID, wsID kernel.WorkspaceID) (*model.ProductWorkflow, kernel.ProjectID, kernel.EnvironmentID, kernel.EnvironmentID) {
	source := kernel.NewEnvironmentID()
	target := kernel.NewEnvironmentID()
	project := kernel.NewProjectID()
	return &model.ProductWorkflow{
		ID: kernel.NewProductWorkflowID(), ProductID: productID, WorkspaceID: wsID,
		Status: model.WorkflowStatusActive,
		Stages: []model.WorkflowStage{{
			Name: "sync",
			Jobs: []model.WorkflowJob{
				{
					Name: "sync-a", Type: model.JobTypeSparkopsEnvironmentSync,
					Spec: map[string]any{
						"source_environment_id": source.String(),
						"target_environment_id": target.String(),
						"projects": []any{map[string]any{
							"project_id": project.String(), "sync_image": true, "sync_yaml": false,
						}},
					},
				},
				{
					Name: "sync-b", Type: model.JobTypeSparkopsEnvironmentSync,
					Spec: map[string]any{
						"source_environment_id": source.String(),
						"target_environment_id": target.String(),
						"projects": []any{map[string]any{
							"project_id": project.String(), "sync_image": true, "sync_yaml": false,
						}},
					},
				},
			},
		}},
	}, project, source, target
}

func TestEnvironmentSyncPreviewBindingRequiresPreviewPerJob(t *testing.T) {
	productID := kernel.NewProductID()
	wsID := kernel.NewWorkspaceID()
	wf, project, source, target := environmentSyncWorkflow(productID, wsID)
	snapshotA := kernel.NewEnvironmentSyncSnapshotID()
	snapshotB := kernel.NewEnvironmentSyncSnapshotID()
	snapshots := &memoryEnvironmentSyncSnapshots{byID: map[string]*appctx.EnvironmentSyncSnapshot{
		snapshotA.String(): {
			ID: snapshotA, ProductID: productID, WorkflowID: wf.ID, JobName: "sync-a",
			SourceEnvironmentID: source, TargetEnvironmentID: target,
			Projects: []appctx.EnvironmentSyncProjectSnapshot{{
				ProjectID: project,
				Option:    appctx.EnvironmentSyncProjectOption{ProjectID: project, SyncImage: true},
			}},
		},
		snapshotB.String(): {
			ID: snapshotB, ProductID: productID, WorkflowID: wf.ID, JobName: "sync-b",
			SourceEnvironmentID: source, TargetEnvironmentID: target,
			Projects: []appctx.EnvironmentSyncProjectSnapshot{{
				ProjectID: project,
				Option:    appctx.EnvironmentSyncProjectOption{ProjectID: project, SyncImage: true},
			}},
		},
	}}
	repo := &startRunWorkflowRepo{}
	handler := NewStartWorkflowRunHandler(repo, engine.NewScheduler(repo, nil, nil, nil, nil, nil), stubProductChecker{wsID: wsID})
	handler.UseEnvironmentSyncSnapshots(snapshots)
	handler.UseTransactor(postgres.NewDirectTransactor())
	handler.scheduler = nil

	_, err := handler.launch(context.Background(), wf, "manual", kernel.NewMemberID(), map[string]any{
		"environment_sync_preview_ids": map[string]any{"sync-a": snapshotA.String()},
	}, nil)
	require.ErrorIs(t, err, domain.ErrInvalidWorkflowInput)

	bindings, err := collectEnvironmentSyncBindings(context.Background(), snapshots, wf, wf.Stages, map[string]any{
		"environment_sync_preview_ids": map[string]any{
			"sync-a": snapshotA.String(),
			"sync-b": snapshotB.String(),
		},
	}, kernel.NewProductWorkflowRunID())
	require.NoError(t, err)
	require.Len(t, bindings, 2)
}

func TestEnvironmentSyncPreviewBindingRejectsMismatchedSnapshot(t *testing.T) {
	productID := kernel.NewProductID()
	wf, project, source, target := environmentSyncWorkflow(productID, kernel.NewWorkspaceID())
	snapshotID := kernel.NewEnvironmentSyncSnapshotID()
	snapshots := &memoryEnvironmentSyncSnapshots{byID: map[string]*appctx.EnvironmentSyncSnapshot{
		snapshotID.String(): {
			ID: snapshotID, ProductID: productID, WorkflowID: wf.ID, JobName: "sync-a",
			SourceEnvironmentID: source, TargetEnvironmentID: target,
			Projects: []appctx.EnvironmentSyncProjectSnapshot{{
				ProjectID: project,
				Option:    appctx.EnvironmentSyncProjectOption{ProjectID: project, SyncImage: true, SyncYAML: true},
			}},
		},
	}}
	_, err := collectEnvironmentSyncBindings(context.Background(), snapshots, wf, wf.Stages, map[string]any{
		"environment_sync_preview_ids": map[string]any{"sync-a": snapshotID.String()},
	}, kernel.NewProductWorkflowRunID())
	require.ErrorIs(t, err, domain.ErrInvalidWorkflowInput)
}

func TestEnvironmentSyncPreviewBindingRejectsAutomaticRunWithoutPreview(t *testing.T) {
	productID := kernel.NewProductID()
	wf, _, _, _ := environmentSyncWorkflow(productID, kernel.NewWorkspaceID())
	handler := NewStartWorkflowRunHandler(&startRunWorkflowRepo{}, nil, stubProductChecker{})
	handler.UseEnvironmentSyncSnapshots(&memoryEnvironmentSyncSnapshots{byID: map[string]*appctx.EnvironmentSyncSnapshot{}})
	handler.UseTransactor(postgres.NewDirectTransactor())
	handler.scheduler = nil

	for _, source := range []string{"git", "cron", "webhook"} {
		_, err := handler.launch(context.Background(), wf, source, kernel.MemberID{}, nil, nil)
		require.ErrorIs(t, err, domain.ErrInvalidWorkflowInput, source)
	}
}

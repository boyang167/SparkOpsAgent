package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type pipelineProjectRepo struct {
	project *model.Project
}

func (r *pipelineProjectRepo) Save(context.Context, *model.Project) error { return nil }
func (r *pipelineProjectRepo) Update(_ context.Context, project *model.Project) error {
	r.project = project
	return nil
}
func (r *pipelineProjectRepo) FindByID(_ context.Context, id kernel.ProjectID) (*model.Project, error) {
	if r.project == nil || r.project.ID != id {
		return nil, domain.ErrProjectNotFound
	}
	return r.project, nil
}
func (r *pipelineProjectRepo) FindByWorkspaceAndName(context.Context, kernel.WorkspaceID, model.ProjectName) (*model.Project, error) {
	return nil, domain.ErrProjectNotFound
}
func (r *pipelineProjectRepo) ListByWorkspace(context.Context, kernel.WorkspaceID) ([]*model.Project, error) {
	return nil, nil
}
func (r *pipelineProjectRepo) ListByProduct(context.Context, kernel.ProductID) ([]*model.Project, error) {
	return nil, nil
}
func (r *pipelineProjectRepo) CountByWorkspace(context.Context, kernel.WorkspaceID) (int, error) {
	return 0, nil
}
func (r *pipelineProjectRepo) CountByProduct(context.Context, kernel.ProductID) (int, error) {
	return 0, nil
}
func (r *pipelineProjectRepo) ListByStatus(context.Context, model.ProjectStatus) ([]*model.Project, error) {
	return nil, nil
}

type pipelineDisabler struct {
	disabled   bool
	disableErr error
	calls      int
}

func (d *pipelineDisabler) PipelineDisabled(context.Context, kernel.WorkspaceID, kernel.ProjectID) (bool, error) {
	return d.disabled, nil
}
func (d *pipelineDisabler) DisablePipeline(context.Context, kernel.WorkspaceID, kernel.ProjectID) error {
	d.calls++
	if d.disableErr == nil {
		d.disabled = true
	}
	return d.disableErr
}

func TestPipelineDisableRetriesWithoutPrematureOrDuplicateArchive(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchiving())
	repo := &pipelineProjectRepo{project: project}
	disabler := &pipelineDisabler{disableErr: errors.New("jenkins unavailable")}
	handler := NewPipelineDisableHandler(func() PipelineDisabler { return disabler }, repo, postgres.NewDirectTransactor())
	payload, err := json.Marshal(PipelineDisablePayload{
		WorkspaceID: project.WorkspaceID.String(), ProjectID: project.ID.String(),
	})
	require.NoError(t, err)
	operation := messaging.Operation{Payload: payload, IdempotencyKey: "event:owner:type:target"}

	require.Error(t, handler(context.Background(), operation))
	require.Equal(t, model.ProjectStatusArchiving, project.Status)

	disabler.disableErr = nil
	require.NoError(t, handler(context.Background(), operation))
	require.Equal(t, model.ProjectStatusArchived, project.Status)
	require.Equal(t, 2, disabler.calls)

	require.NoError(t, handler(context.Background(), operation))
	require.Equal(t, 2, disabler.calls)
}

type txMarkerKey struct{}

type markerTransactor struct{}

func (markerTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(context.WithValue(ctx, txMarkerKey{}, true))
}

type undeployTeardown struct {
	online       []kernel.ClusterInfoID
	active       bool
	starts       int
	calledInside bool
}

func (t *undeployTeardown) mark(ctx context.Context) {
	if ctx.Value(txMarkerKey{}) == true {
		t.calledInside = true
	}
}
func (t *undeployTeardown) OnlineClusters(ctx context.Context, _ kernel.WorkspaceID, _ kernel.ProjectID) ([]kernel.ClusterInfoID, error) {
	t.mark(ctx)
	return t.online, nil
}
func (t *undeployTeardown) HasActiveRun(ctx context.Context, _ kernel.ProjectID) (bool, error) {
	t.mark(ctx)
	return t.active, nil
}
func (t *undeployTeardown) StartUndeploy(ctx context.Context, _ appctx.UndeployRequest) (kernel.DeploymentRunID, error) {
	t.mark(ctx)
	t.starts++
	return kernel.NewDeploymentRunID(), nil
}
func (t *undeployTeardown) DisablePipeline(context.Context, kernel.WorkspaceID, kernel.ProjectID) error {
	return nil
}
func (t *undeployTeardown) PipelineDisabled(context.Context, kernel.WorkspaceID, kernel.ProjectID) (bool, error) {
	return false, nil
}

type undeployOperationWriter struct {
	requests []messaging.OperationRequest
	err      error
}

func (w *undeployOperationWriter) Schedule(_ context.Context, request messaging.OperationRequest) error {
	w.requests = append(w.requests, request)
	return w.err
}

func TestProjectUndeployRetriesUntilOfflineThenSchedulesPipeline(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchiving())
	repo := &pipelineProjectRepo{project: project}
	clusterID := kernel.NewClusterInfoID()
	teardown := &undeployTeardown{online: []kernel.ClusterInfoID{clusterID}}
	writer := &undeployOperationWriter{}
	handler := NewProjectUndeployHandler(
		repo, func() appctx.ProjectTeardown { return teardown }, writer, markerTransactor{},
	)
	payload, err := json.Marshal(ProjectUndeployPayload{
		ProjectID: project.ID.String(), MemberID: project.CreatedBy.String(),
		AllowUndeploy: boolRef(true),
	})
	require.NoError(t, err)
	operation := messaging.Operation{
		EventID: "event-1", Owner: "workspace-app.archive-resumer", Payload: payload,
	}

	err = handler(context.Background(), operation)
	require.Error(t, err)
	require.False(t, messaging.IsPermanent(err))
	require.Equal(t, 1, teardown.starts)
	require.Empty(t, writer.requests)
	require.False(t, teardown.calledInside)

	// The completion event is lost; a retry reconciles external state instead.
	teardown.online = nil
	require.NoError(t, handler(context.Background(), operation))
	require.Len(t, writer.requests, 1)
	require.Equal(t, OperationTypePipelineDisable, writer.requests[0].OperationType)
	require.Equal(t, model.ProjectStatusArchiving, project.Status)
	require.False(t, teardown.calledInside)
}

func TestProjectUndeployRetriesPipelineScheduleAfterExternalStateConverges(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchiving())
	repo := &pipelineProjectRepo{project: project}
	teardown := &undeployTeardown{}
	writer := &undeployOperationWriter{err: errors.New("outbox unavailable")}
	handler := NewProjectUndeployHandler(
		repo, func() appctx.ProjectTeardown { return teardown }, writer, markerTransactor{},
	)
	payload, err := json.Marshal(ProjectUndeployPayload{
		ProjectID: project.ID.String(), MemberID: project.CreatedBy.String(),
		AllowUndeploy: boolRef(true),
	})
	require.NoError(t, err)
	operation := messaging.Operation{EventID: "event-1", Owner: "workspace-app.archive-resumer", Payload: payload}

	require.Error(t, handler(context.Background(), operation))
	require.Equal(t, model.ProjectStatusArchiving, project.Status)
	writer.err = nil
	require.NoError(t, handler(context.Background(), operation))
	require.Len(t, writer.requests, 2)
	require.False(t, teardown.calledInside)
}

func boolRef(value bool) *bool {
	return &value
}

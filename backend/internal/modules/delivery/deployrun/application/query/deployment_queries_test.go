package query

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/stream"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type queryRepo struct {
	run              *model.DeploymentRun
	runs             []*model.DeploymentRun
	events           []model.DeploymentRunEvent
	beforeListEvents func()
}

func (*queryRepo) Create(context.Context, *model.DeploymentRun) error { return nil }
func (*queryRepo) Update(context.Context, *model.DeploymentRun) error { return nil }
func (r *queryRepo) FindByID(context.Context, kernel.DeploymentRunID) (*model.DeploymentRun, error) {
	if r.run == nil {
		return nil, domain.ErrDeploymentRunNotFound
	}
	return r.run, nil
}
func (*queryRepo) FindByBuildID(context.Context, kernel.BuildID) (*model.DeploymentRun, error) {
	return nil, domain.ErrDeploymentRunNotFound
}
func (r *queryRepo) ListByProject(context.Context, kernel.ProjectID, int) ([]*model.DeploymentRun, error) {
	return r.runs, nil
}
func (r *queryRepo) ListPageByProject(_ context.Context, wsID kernel.WorkspaceID, _ kernel.ProjectID, page, pageSize int) (kernel.Page[*model.DeploymentRun], error) {
	filtered := make([]*model.DeploymentRun, 0, len(r.runs))
	for _, run := range r.runs {
		if run.WorkspaceID == wsID {
			filtered = append(filtered, run)
		}
	}
	return kernel.NewPage(filtered, int64(len(filtered)), page, pageSize), nil
}
func (r *queryRepo) ListActive(context.Context) ([]*model.DeploymentRun, error) {
	return r.runs, nil
}
func (*queryRepo) TryAcquireLease(context.Context, kernel.DeploymentRunID, int64, string, time.Duration) (bool, error) {
	return true, nil
}
func (*queryRepo) ReleaseLease(context.Context, kernel.DeploymentRunID, int64, string) error {
	return nil
}
func (*queryRepo) RecordInfrastructureFailure(context.Context, kernel.DeploymentRunID, int64, string) (*model.DeploymentRun, error) {
	return nil, nil
}
func (r *queryRepo) ListEvents(context.Context, kernel.DeploymentRunID) ([]model.DeploymentRunEvent, error) {
	if r.beforeListEvents != nil {
		r.beforeListEvents()
	}
	return r.events, nil
}
func (*queryRepo) HasActiveByProject(context.Context, kernel.ProjectID) (bool, error) {
	return false, nil
}

type optionsReader struct {
	clusters []port.DeploymentCluster
	errs     map[string]error
}

func (r optionsReader) Load(_ context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID, clusterID kernel.ClusterInfoID) (port.ProjectDeploymentContext, error) {
	if err := r.errs[clusterID.String()]; err != nil {
		return port.ProjectDeploymentContext{}, err
	}
	return port.ProjectDeploymentContext{WorkspaceID: wsID, ProjectID: projectID, Cluster: port.DeploymentCluster{ID: clusterID}}, nil
}
func (r optionsReader) ListAuthorizedClusters(context.Context, kernel.WorkspaceID) ([]port.DeploymentCluster, error) {
	return r.clusters, nil
}

func TestGetDeploymentRejectsOtherWorkspace(t *testing.T) {
	run := model.NewDeploymentRun(kernel.NewWorkspaceID(), kernel.NewProjectID(), kernel.NewClusterInfoID(),
		kernel.NewMemberID(), "branch", "main", "")

	_, err := NewGetDeploymentHandler(&queryRepo{run: run}).Handle(
		context.Background(), kernel.NewWorkspaceID(), run.ID)

	require.ErrorIs(t, err, domain.ErrDeploymentRunNotFound)
}

func TestListDeploymentsOnlyReturnsWorkspaceRuns(t *testing.T) {
	wsID, projectID := kernel.NewWorkspaceID(), kernel.NewProjectID()
	own := model.NewDeploymentRun(wsID, projectID, kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")
	other := model.NewDeploymentRun(kernel.NewWorkspaceID(), projectID, kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")

	page, err := NewListDeploymentsHandler(&queryRepo{runs: []*model.DeploymentRun{own, other}}).
		Handle(context.Background(), wsID, projectID)

	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, 1, page.Page)
	require.Equal(t, 20, page.PageSize)
	require.Len(t, page.Items, 1)
	require.Equal(t, own.ID.String(), page.Items[0].ID)
}

func TestDeploymentOptionsExposePerClusterGateErrors(t *testing.T) {
	wsID, projectID := kernel.NewWorkspaceID(), kernel.NewProjectID()
	ready, blocked := kernel.NewClusterInfoID(), kernel.NewClusterInfoID()
	reader := optionsReader{
		clusters: []port.DeploymentCluster{{ID: ready, Name: "ready"}, {ID: blocked, Name: "blocked"}},
		errs:     map[string]error{blocked.String(): domain.ErrKubernetesKubeconfigMissing},
	}

	options, err := NewDeploymentOptionsHandler(reader).Handle(context.Background(), wsID, projectID)

	require.NoError(t, err)
	require.Len(t, options.Clusters, 2)
	require.True(t, options.Clusters[0].Available)
	require.False(t, options.Clusters[1].Available)
	require.Equal(t, "DELIVERY_KUBECONFIG_MISSING", options.Clusters[1].ReasonCode)
}

func TestStreamDeploymentReturnsUnsubscribeForActiveRun(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(),
		kernel.NewMemberID(), "branch", "main", "")
	handler := NewStreamDeploymentHandler(&queryRepo{run: run}, stream.NewHub())

	ch, _, done, unsubscribe, err := handler.Subscribe(context.Background(), wsID, run.ID)

	require.NoError(t, err)
	require.False(t, done)
	unsubscribe()
	_, open := <-ch
	require.False(t, open)
}

func TestStreamDeploymentReturnsDoneWithoutHubSubscriptionForTerminalRun(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(),
		kernel.NewMemberID(), "branch", "main", "")
	run.Fail("failed", run.CreatedAt)
	event := model.DeploymentRunEvent{
		ID: 1, RunID: run.ID, At: run.CreatedAt, Status: model.StatusFailed, Message: "failed",
	}
	handler := NewStreamDeploymentHandler(&queryRepo{run: run, events: []model.DeploymentRunEvent{event}}, stream.NewHub())

	ch, initial, done, unsubscribe, err := handler.Subscribe(context.Background(), wsID, run.ID)

	require.NoError(t, err)
	require.True(t, done)
	require.Contains(t, initial, `"status":"failed"`)
	require.NotNil(t, unsubscribe)
	unsubscribe()
	_, open := <-ch
	require.False(t, open)
}

func TestStreamDeploymentSubscribesBeforeSnapshotWithoutDuplicatingRacingEvent(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(),
		kernel.NewMemberID(), "branch", "main", "")
	hub := stream.NewHub()
	queued := model.DeploymentRunEvent{
		ID: 1, RunID: run.ID, At: run.CreatedAt, Status: model.StatusQueued, Message: "deployment queued",
	}
	preparing := model.DeploymentRunEvent{
		ID: 2, RunID: run.ID, At: run.CreatedAt.Add(time.Second),
		Status: model.StatusPreparing, Stage: model.StagePreparing, Message: "deployment preparing",
	}
	repo := &queryRepo{run: run, events: []model.DeploymentRunEvent{queued, preparing}}
	repo.beforeListEvents = func() { hub.PublishEvent(run, preparing) }
	handler := NewStreamDeploymentHandler(repo, hub)

	ch, initial, done, unsubscribe, err := handler.Subscribe(context.Background(), wsID, run.ID)
	require.NoError(t, err)
	defer unsubscribe()
	require.False(t, done)
	require.Equal(t, 1, strings.Count(initial, `"status":"queued"`))
	require.Equal(t, 1, strings.Count(initial, `"status":"preparing"`))

	building := model.DeploymentRunEvent{
		ID: 3, RunID: run.ID, At: run.CreatedAt.Add(2 * time.Second),
		Status: model.StatusBuilding, Stage: model.StageBuilding, Message: "deployment building",
	}
	hub.PublishEvent(run, building)
	require.Contains(t, <-ch, `"status":"building"`)
	select {
	case duplicate := <-ch:
		t.Fatalf("unexpected duplicate snapshot/live event: %q", duplicate)
	default:
	}
}

func TestStreamDeploymentRecoversCompleteTerminalSnapshotAfterHubOverflow(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(),
		kernel.NewMemberID(), "branch", "main", "")
	hub := stream.NewHub()
	slow, _, _, unsubscribeSlow := hub.Subscribe(run.ID)
	defer unsubscribeSlow()
	events := make([]model.DeploymentRunEvent, 0, 34)
	for index := 1; index <= 33; index++ {
		event := model.DeploymentRunEvent{
			ID: uint64(index), RunID: run.ID, At: run.CreatedAt.Add(time.Duration(index) * time.Second),
			Status: model.StatusBuilding, Stage: model.StageBuilding, Message: "build progress",
		}
		events = append(events, event)
		hub.PublishEvent(run, event)
	}
	for range slow {
	}
	run.Fail("ci failed", run.CreatedAt.Add(34*time.Second))
	events = append(events, model.DeploymentRunEvent{
		ID: 34, RunID: run.ID, At: *run.FinishedAt, Status: model.StatusFailed,
		Stage: model.StageBuilding, Message: "ci failed",
	})
	handler := NewStreamDeploymentHandler(&queryRepo{run: run, events: events}, hub)

	ch, initial, done, unsubscribe, err := handler.Subscribe(context.Background(), wsID, run.ID)
	require.NoError(t, err)
	defer unsubscribe()
	require.True(t, done)
	require.Equal(t, 34, strings.Count(initial, `"event_id"`))
	require.Contains(t, initial, `"status":"failed"`)
	_, open := <-ch
	require.False(t, open)
}

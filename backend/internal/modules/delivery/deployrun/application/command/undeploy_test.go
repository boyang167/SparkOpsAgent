package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type mutableArchiveFence struct {
	value port.ProjectLifecycle
}

func (f *mutableArchiveFence) ProjectLifecycle(context.Context, kernel.WorkspaceID, kernel.ProjectID) (port.ProjectLifecycle, error) {
	return f.value, nil
}

func (f *mutableArchiveFence) TransitionArchivePhase(
	_ context.Context,
	_ kernel.ProjectID,
	generation string,
	expected string,
	target string,
) (bool, error) {
	if f.value.Status != "archiving" ||
		f.value.ArchiveGeneration != generation ||
		f.value.ArchivePhase != expected {
		return false, nil
	}
	f.value.ArchivePhase = target
	return true, nil
}

type archiveOperationWriter struct {
	err      error
	requests []messaging.OperationRequest
}

func (w *archiveOperationWriter) Schedule(_ context.Context, request messaging.OperationRequest) error {
	w.requests = append(w.requests, request)
	return w.err
}

type archiveTransaction struct {
	fence *mutableArchiveFence
	repo  *fakeRunRepo
}

func (t archiveTransaction) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	phase := t.fence.value.ArchivePhase
	created := len(t.repo.created)
	err := fn(ctx)
	if err != nil {
		t.fence.value.ArchivePhase = phase
		t.repo.created = t.repo.created[:created]
	}
	return err
}

type fakeLifecycle struct {
	value port.ProjectLifecycle
	err   error
}

func (f fakeLifecycle) ProjectLifecycle(context.Context, kernel.WorkspaceID, kernel.ProjectID) (port.ProjectLifecycle, error) {
	return f.value, f.err
}

func (f fakeLifecycle) TransitionArchivePhase(
	_ context.Context,
	_ kernel.ProjectID,
	generation string,
	expected string,
	target string,
) (bool, error) {
	return f.err == nil &&
		f.value.Status == "archiving" &&
		f.value.ArchiveGeneration == generation &&
		f.value.ArchivePhase == expected &&
		target == "undeploying", f.err
}

func onlineRun(wsID kernel.WorkspaceID, projectID kernel.ProjectID, clusterID kernel.ClusterInfoID) *model.DeploymentRun {
	run := model.NewDeploymentRun(wsID, projectID, clusterID, kernel.NewMemberID(), "branch", "main", "v1")
	run.Status = model.StatusSucceeded
	run.DeployNamespace = "team-a-dev"
	return run
}

func newUndeployFixture(status string) (*UndeployHandler, *fakeRunRepo, *recordingQueue, kernel.WorkspaceID, kernel.ProjectID, kernel.ClusterInfoID) {
	wsID := kernel.NewWorkspaceID()
	projectID := kernel.NewProjectID()
	clusterID := kernel.NewClusterInfoID()
	repo := &fakeRunRepo{byProject: []*model.DeploymentRun{onlineRun(wsID, projectID, clusterID)}}
	queue := &recordingQueue{}
	handler := NewUndeployHandler(repo, fakeLifecycle{value: port.ProjectLifecycle{Name: "orders", Status: status}}, queue, nil)
	return handler, repo, queue, wsID, projectID, clusterID
}

func TestUndeployCreatesRunFromOnlineBaseline(t *testing.T) {
	handler, repo, queue, wsID, projectID, clusterID := newUndeployFixture("active")

	run, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, UndeployInput{
		ClusterID: clusterID.String(), ConfirmName: "orders", DeleteStorage: true,
	})

	require.NoError(t, err)
	require.Equal(t, model.RunTypeUndeploy, run.RunType)
	require.Equal(t, clusterID, run.ClusterID)
	require.Equal(t, "team-a-dev", run.DeployNamespace)
	require.True(t, run.UndeployOptions.DeleteStorage)
	require.Len(t, repo.created, 1)
	require.Len(t, queue.payloads, 1)
}

func TestUndeployRejectsConfirmNameMismatch(t *testing.T) {
	handler, repo, _, wsID, projectID, clusterID := newUndeployFixture("active")
	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, UndeployInput{
		ClusterID: clusterID.String(), ConfirmName: "order",
	})
	require.ErrorIs(t, err, domain.ErrConfirmNameMismatch)
	require.Empty(t, repo.created)
}

func TestUndeployRejectsProjectBoundToProduct(t *testing.T) {
	handler, repo, _, wsID, projectID, clusterID := newUndeployFixture("active")
	handler.lifecycle = fakeLifecycle{value: port.ProjectLifecycle{Name: "orders", Status: "active", ProductID: kernel.NewProductID().String()}}

	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, UndeployInput{
		ClusterID: clusterID.String(), ConfirmName: "orders",
	})

	require.ErrorIs(t, err, domain.ErrProjectBoundToProduct)
	require.Empty(t, repo.created)
}

func TestUndeployRejectsWhenRunActive(t *testing.T) {
	handler, repo, _, wsID, projectID, clusterID := newUndeployFixture("active")
	repo.active = true
	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, UndeployInput{
		ClusterID: clusterID.String(), ConfirmName: "orders",
	})
	require.ErrorIs(t, err, domain.ErrDeploymentAlreadyRunning)
}

func TestUndeployRejectsOfflineCluster(t *testing.T) {
	handler, _, _, wsID, projectID, _ := newUndeployFixture("active")
	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, UndeployInput{
		ClusterID: kernel.NewClusterInfoID().String(), ConfirmName: "orders",
	})
	require.ErrorIs(t, err, domain.ErrEnvironmentNotDeployed)
}

func TestUndeployRejectsAlreadyUndeployedCluster(t *testing.T) {
	handler, repo, _, wsID, projectID, clusterID := newUndeployFixture("active")
	previous := model.NewUndeployRun(wsID, projectID, kernel.NewMemberID(), repo.byProject[0], model.UndeployOptions{})
	previous.Status = model.StatusSucceeded
	repo.byProject = append([]*model.DeploymentRun{previous}, repo.byProject...)

	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, UndeployInput{
		ClusterID: clusterID.String(), ConfirmName: "orders",
	})
	require.ErrorIs(t, err, domain.ErrEnvironmentNotDeployed)
}

func TestUndeployRejectsArchivingProjectButStartForArchiveAllows(t *testing.T) {
	handler, _, _, wsID, projectID, clusterID := newUndeployFixture("archiving")
	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, UndeployInput{
		ClusterID: clusterID.String(), ConfirmName: "orders",
	})
	require.ErrorIs(t, err, domain.ErrProjectArchiving)

	generation := uuid.NewString()
	handler.lifecycle = fakeLifecycle{value: port.ProjectLifecycle{
		Name: "orders", Status: "archiving",
		ArchiveGeneration: generation, ArchivePhase: "undeploy_claimed",
	}}
	run, err := handler.StartForArchive(context.Background(), wsID, kernel.NewMemberID(), projectID, clusterID, generation)
	require.NoError(t, err)
	require.Equal(t, model.RunTypeUndeploy, run.RunType)
	require.Equal(t, &model.UndeployOptions{}, run.UndeployOptions)
	require.Equal(t, generation, run.ArchiveGeneration)
}

func TestStartForArchiveRejectsActiveProjectEvenWithGeneration(t *testing.T) {
	handler, repo, _, wsID, projectID, clusterID := newUndeployFixture("active")

	_, err := handler.StartForArchive(
		context.Background(), wsID, kernel.NewMemberID(), projectID, clusterID, uuid.NewString(),
	)

	require.Error(t, err)
	require.Empty(t, repo.created)
}

func TestStartForArchiveAtomicallyBindsFenceRunAndQueueOperation(t *testing.T) {
	_, repo, queue, wsID, projectID, clusterID := newUndeployFixture("archiving")
	generation := uuid.NewString()
	fence := &mutableArchiveFence{value: port.ProjectLifecycle{
		Name: "orders", Status: "archiving",
		ArchiveGeneration: generation, ArchivePhase: archivePhaseClaimed,
	}}
	handler := NewUndeployHandler(repo, fence, queue, nil)
	writer := &archiveOperationWriter{err: errors.New("outbox unavailable")}
	handler.UseOperationOutbox(writer, archiveTransaction{fence: fence, repo: repo})

	_, err := handler.StartForArchive(
		context.Background(), wsID, kernel.NewMemberID(), projectID, clusterID, generation,
	)
	require.Error(t, err)
	require.Equal(t, archivePhaseClaimed, fence.value.ArchivePhase)
	require.Empty(t, repo.created)

	writer.err = nil
	run, err := handler.StartForArchive(
		context.Background(), wsID, kernel.NewMemberID(), projectID, clusterID, generation,
	)
	require.NoError(t, err)
	require.Equal(t, archivePhaseUndeploying, fence.value.ArchivePhase)
	require.Len(t, repo.created, 1)
	require.Equal(t, generation, run.ArchiveGeneration)
	require.Len(t, writer.requests, 2)
}

func TestCreateDeploymentRejectsArchivingProject(t *testing.T) {
	repo := &fakeRunRepo{}
	handler := NewCreateDeploymentHandler(repo, fakeDeploymentReader{}, &recordingQueue{}, nil)
	handler.SetProjectLifecycleReader(fakeLifecycle{value: port.ProjectLifecycle{Name: "orders", Status: "archiving"}})

	_, err := handler.Handle(context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), kernel.NewProjectID(), CreateInput{
		GitRefType: "branch", GitRefValue: "main",
	})

	require.ErrorIs(t, err, domain.ErrProjectArchiving)
	require.Empty(t, repo.created)
}

func TestRollbackDeploymentRejectsArchivingProject(t *testing.T) {
	handler := NewRollbackDeploymentHandler(&fakeRunRepo{}, &recordingQueue{}, nil)
	handler.SetProjectLifecycleReader(fakeLifecycle{value: port.ProjectLifecycle{Name: "orders", Status: "archiving"}})

	_, err := handler.Handle(context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), kernel.NewProjectID(), RollbackInput{})

	require.ErrorIs(t, err, domain.ErrProjectArchiving)
}

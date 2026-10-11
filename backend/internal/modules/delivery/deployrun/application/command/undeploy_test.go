package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type fakeLifecycle struct {
	value port.ProjectLifecycle
	err   error
}

func (f fakeLifecycle) ProjectLifecycle(context.Context, kernel.WorkspaceID, kernel.ProjectID) (port.ProjectLifecycle, error) {
	return f.value, f.err
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

	run, err := handler.StartForArchive(context.Background(), wsID, kernel.NewMemberID(), projectID, clusterID)
	require.NoError(t, err)
	require.Equal(t, model.RunTypeUndeploy, run.RunType)
	require.Equal(t, &model.UndeployOptions{}, run.UndeployOptions)
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

package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type archiveProjectRepo struct {
	projects map[kernel.ProjectID]*model.Project
	updates  int
}

func (r *archiveProjectRepo) Save(context.Context, *model.Project) error { return nil }
func (r *archiveProjectRepo) Update(_ context.Context, p *model.Project) error {
	r.updates++
	r.projects[p.ID] = p
	return nil
}
func (r *archiveProjectRepo) FindByID(_ context.Context, id kernel.ProjectID) (*model.Project, error) {
	p, ok := r.projects[id]
	if !ok {
		return nil, domain.ErrProjectNotFound
	}
	return p, nil
}
func (r *archiveProjectRepo) FindByWorkspaceAndName(context.Context, kernel.WorkspaceID, model.ProjectName) (*model.Project, error) {
	return nil, domain.ErrProjectNotFound
}
func (r *archiveProjectRepo) ListByWorkspace(context.Context, kernel.WorkspaceID) ([]*model.Project, error) {
	return nil, nil
}
func (r *archiveProjectRepo) ListByProduct(context.Context, kernel.ProductID) ([]*model.Project, error) {
	return nil, nil
}
func (r *archiveProjectRepo) CountByWorkspace(context.Context, kernel.WorkspaceID) (int, error) {
	return 0, nil
}
func (r *archiveProjectRepo) CountByProduct(context.Context, kernel.ProductID) (int, error) {
	return 0, nil
}
func (r *archiveProjectRepo) ListByStatus(_ context.Context, status model.ProjectStatus) ([]*model.Project, error) {
	out := []*model.Project{}
	for _, p := range r.projects {
		if p.Status == status {
			out = append(out, p)
		}
	}
	return out, nil
}

type fakeTeardown struct {
	online     []kernel.ClusterInfoID
	active     bool
	started    []kernel.ClusterInfoID
	disabled   int
	disableErr error
	startErr   error
}

func (f *fakeTeardown) OnlineClusters(context.Context, kernel.WorkspaceID, kernel.ProjectID) ([]kernel.ClusterInfoID, error) {
	return f.online, nil
}
func (f *fakeTeardown) HasActiveRun(context.Context, kernel.ProjectID) (bool, error) {
	return f.active, nil
}
func (f *fakeTeardown) StartUndeploy(_ context.Context, req appctx.UndeployRequest) (kernel.DeploymentRunID, error) {
	if f.startErr != nil {
		return kernel.DeploymentRunID{}, f.startErr
	}
	f.started = append(f.started, req.ClusterID)
	return kernel.NewDeploymentRunID(), nil
}
func (f *fakeTeardown) DisablePipeline(context.Context, kernel.WorkspaceID, kernel.ProjectID) error {
	f.disabled++
	return f.disableErr
}

func archiveFixture(t *testing.T, online ...kernel.ClusterInfoID) (*command.ArchiveProjectHandler, *command.ArchiveCoordinator, *fakeTeardown, *model.Project) {
	t.Helper()
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	repo := &archiveProjectRepo{projects: map[kernel.ProjectID]*model.Project{project.ID: project}}
	teardown := &fakeTeardown{online: online}
	coordinator := command.NewArchiveCoordinator(repo, func() appctx.ProjectTeardown { return teardown }, func(string, error) {})
	return command.NewArchiveProjectHandler(repo, updateWorkspaceChecker{}, coordinator), coordinator, teardown, project
}

func undeployEvent(project *model.Project, status string) deliveryevent.DeployCompleted {
	ev := deliveryevent.NewDeployCompleted(kernel.NewDeploymentRunID(), project.ID, project.WorkspaceID, nil, "", status, "undeploy", "v1")
	ev.CreatedBy = kernel.NewMemberID()
	return ev
}

func TestArchiveWithoutOnlineEnvironmentsArchivesImmediately(t *testing.T) {
	handler, _, teardown, project := archiveFixture(t)

	result, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders"})

	require.NoError(t, err)
	require.Equal(t, "archived", result.Status)
	require.Empty(t, result.PendingClusters)
	require.Equal(t, model.ProjectStatusArchived, project.Status)
	require.Equal(t, 1, teardown.disabled)
}

func TestArchiveRequiresTeardownWhenOnline(t *testing.T) {
	handler, _, _, project := archiveFixture(t, kernel.NewClusterInfoID())

	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders"})

	require.ErrorIs(t, err, domain.ErrProjectHasOnlineEnvironments)
	require.Equal(t, model.ProjectStatusActive, project.Status)
}

func TestArchiveRejectsConfirmMismatch(t *testing.T) {
	handler, _, _, project := archiveFixture(t)

	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "x"})

	require.ErrorIs(t, err, domain.ErrConfirmNameMismatch)
}

func TestArchiveRejectsWhileDeploymentRunning(t *testing.T) {
	handler, _, teardown, project := archiveFixture(t)
	teardown.active = true

	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders"})

	require.ErrorIs(t, err, domain.ErrProjectDeploymentRunning)
}

func TestArchiveWithTeardownStartsFirstUndeploy(t *testing.T) {
	dev, uat := kernel.NewClusterInfoID(), kernel.NewClusterInfoID()
	handler, _, teardown, project := archiveFixture(t, dev, uat)

	result, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders", Teardown: true})

	require.NoError(t, err)
	require.Equal(t, "archiving", result.Status)
	require.Equal(t, []string{dev.String(), uat.String()}, result.PendingClusters)
	require.Equal(t, model.ProjectStatusArchiving, project.Status)
	require.Equal(t, []kernel.ClusterInfoID{dev}, teardown.started)
}

func TestArchiveRevertsWhenFirstUndeployCannotStart(t *testing.T) {
	handler, _, teardown, project := archiveFixture(t, kernel.NewClusterInfoID())
	teardown.startErr = errors.New("queue down")

	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders", Teardown: true})

	require.Error(t, err)
	require.Equal(t, model.ProjectStatusActive, project.Status)
}

func TestArchiveRejectsProjectAlreadyArchiving(t *testing.T) {
	handler, _, _, project := archiveFixture(t, kernel.NewClusterInfoID())
	require.NoError(t, project.BeginArchiving())

	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders", Teardown: true})

	require.ErrorIs(t, err, domain.ErrProjectArchiving)
}

func TestCoordinatorAdvancesOnUndeploySuccessAndFinishes(t *testing.T) {
	dev, uat := kernel.NewClusterInfoID(), kernel.NewClusterInfoID()
	handler, coordinator, teardown, project := archiveFixture(t, dev, uat)
	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders", Teardown: true})
	require.NoError(t, err)

	teardown.online = []kernel.ClusterInfoID{uat}
	require.NoError(t, coordinator.HandleDeployCompleted(context.Background(), undeployEvent(project, "succeeded")))
	require.Equal(t, []kernel.ClusterInfoID{dev, uat}, teardown.started)
	require.Equal(t, model.ProjectStatusArchiving, project.Status)

	teardown.online = nil
	require.NoError(t, coordinator.HandleDeployCompleted(context.Background(), undeployEvent(project, "succeeded")))
	require.Equal(t, model.ProjectStatusArchived, project.Status)
	require.Equal(t, 1, teardown.disabled)
}

func TestCoordinatorIgnoresNormalDeployEvents(t *testing.T) {
	handler, coordinator, teardown, project := archiveFixture(t, kernel.NewClusterInfoID())
	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders", Teardown: true})
	require.NoError(t, err)
	ev := undeployEvent(project, "failed")
	ev.RunType = "normal"

	require.NoError(t, coordinator.HandleDeployCompleted(context.Background(), ev))

	require.Equal(t, model.ProjectStatusArchiving, project.Status)
	require.Len(t, teardown.started, 1)
}

func TestCoordinatorWaitsWhenAnotherUndeployIsAlreadyRunning(t *testing.T) {
	dev, uat := kernel.NewClusterInfoID(), kernel.NewClusterInfoID()
	handler, coordinator, teardown, project := archiveFixture(t, dev, uat)
	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders", Teardown: true})
	require.NoError(t, err)
	teardown.startErr = kernel.NewDomainError("DELIVERY_DEPLOYMENT_RUNNING", "a deployment is already running for this project")

	require.NoError(t, coordinator.HandleDeployCompleted(context.Background(), undeployEvent(project, "succeeded")))

	require.Equal(t, model.ProjectStatusArchiving, project.Status)
}

func TestCoordinatorAbortsArchivingOnUndeployFailure(t *testing.T) {
	handler, coordinator, _, project := archiveFixture(t, kernel.NewClusterInfoID())
	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders", Teardown: true})
	require.NoError(t, err)

	require.NoError(t, coordinator.HandleDeployCompleted(context.Background(), undeployEvent(project, "failed")))

	require.Equal(t, model.ProjectStatusActive, project.Status)
}

func TestCoordinatorArchivesEvenWhenDisableFails(t *testing.T) {
	handler, _, teardown, project := archiveFixture(t)
	teardown.disableErr = errors.New("jenkins down")

	result, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders"})

	require.NoError(t, err)
	require.Equal(t, "archived", result.Status)
}

func TestLegacyArchiveRejectsOnlineProject(t *testing.T) {
	handler, _, _, project := archiveFixture(t, kernel.NewClusterInfoID())

	err := handler.HandleLegacy(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID)

	require.ErrorIs(t, err, domain.ErrProjectHasOnlineEnvironments)
}

func TestLegacyArchiveArchivesOfflineProject(t *testing.T) {
	handler, _, _, project := archiveFixture(t)

	require.NoError(t, handler.HandleLegacy(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID))

	require.Equal(t, model.ProjectStatusArchived, project.Status)
}

func TestResumeAllRestartsStalledArchiving(t *testing.T) {
	dev := kernel.NewClusterInfoID()
	_, coordinator, teardown, project := archiveFixture(t, dev)
	require.NoError(t, project.BeginArchiving())

	require.NoError(t, coordinator.ResumeAll(context.Background()))

	require.Equal(t, []kernel.ClusterInfoID{dev}, teardown.started)
}

func TestResumeAllWaitsForActiveRun(t *testing.T) {
	_, coordinator, teardown, project := archiveFixture(t, kernel.NewClusterInfoID())
	require.NoError(t, project.BeginArchiving())
	teardown.active = true

	require.NoError(t, coordinator.ResumeAll(context.Background()))

	require.Empty(t, teardown.started)
	require.Equal(t, model.ProjectStatusArchiving, project.Status)
}

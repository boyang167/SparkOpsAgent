package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	jenkinsinfra "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/jenkins"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/runner"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/stream"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type triggerBuildRepo struct {
	saves     int
	updates   int
	saved     *model.Build
	updateErr error
	failAfter int
	onUpdate  func(int)
}

func (r *triggerBuildRepo) Save(_ context.Context, build *model.Build) error {
	r.saves++
	r.saved = build
	return nil
}
func (r *triggerBuildRepo) Update(context.Context, *model.Build) error {
	r.updates++
	if r.onUpdate != nil {
		r.onUpdate(r.updates)
	}
	if r.updateErr != nil && r.updates > r.failAfter {
		return r.updateErr
	}
	return nil
}
func (*triggerBuildRepo) FindByID(context.Context, kernel.BuildID) (*model.Build, error) {
	return nil, nil
}
func (*triggerBuildRepo) ListByProject(context.Context, kernel.ProjectID, int) ([]*model.Build, error) {
	return nil, nil
}
func (*triggerBuildRepo) HasRunningBuild(context.Context, kernel.ProjectID) (bool, error) {
	return false, nil
}
func (*triggerBuildRepo) NextBuildNumber(context.Context, kernel.ProjectID) (int, error) {
	return 7, nil
}

type triggerProjectChecker struct {
	namespace string
}

func (triggerProjectChecker) ProjectInWorkspace(context.Context, kernel.ProjectID, kernel.WorkspaceID) (bool, error) {
	return true, nil
}
func (triggerProjectChecker) ProjectIsActive(context.Context, kernel.ProjectID) (bool, error) {
	return true, nil
}
func (triggerProjectChecker) ProjectName(context.Context, kernel.ProjectID) (string, error) {
	return "orders", nil
}
func (c triggerProjectChecker) ProjectNamespace(context.Context, kernel.ProjectID) (string, error) {
	return c.namespace, nil
}
func (triggerProjectChecker) ProjectAutomationContext(context.Context, kernel.ProjectID) (port.ProjectAutomationContext, error) {
	return port.ProjectAutomationContext{}, nil
}

type requestCapturingJenkins struct {
	requests chan port.BuildRequest
}

func (j *requestCapturingJenkins) CreateOrUpdate(context.Context, port.JenkinsJobSpec) error {
	return nil
}
func (j *requestCapturingJenkins) SyncCredentials(context.Context, string, []port.JenkinsCredential) error {
	return nil
}
func (j *requestCapturingJenkins) RunStreaming(_ context.Context, _ kernel.BuildID, req port.BuildRequest, _ port.LogPublisher) (string, error) {
	j.requests <- req
	return "ok", nil
}

func newTriggerHandler(executor port.JenkinsExecutor) *TriggerBuildHandler {
	return newTriggerHandlerWithRepo(&triggerBuildRepo{}, executor)
}

func newTriggerHandlerWithRepo(repo *triggerBuildRepo, executor port.JenkinsExecutor) *TriggerBuildHandler {
	return NewTriggerBuildHandler(
		repo,
		triggerProjectChecker{namespace: "team-a"},
		jenkinsinfra.NewStaticProvider(executor),
		eventbus.NewInMemory(),
		stream.NewHub(),
		runner.NewRegistry(),
		context.Background(),
		postgres.NewDirectTransactor(),
	)
}

func TestHandleKeepsOrdinaryBuildJobAndBranchCompatible(t *testing.T) {
	jenkins := &requestCapturingJenkins{requests: make(chan port.BuildRequest, 1)}
	handler := newTriggerHandler(jenkins)

	_, err := handler.Handle(
		context.Background(),
		kernel.NewWorkspaceID(),
		kernel.NewMemberID(),
		kernel.NewProjectID(),
		"feature/orders",
	)
	require.NoError(t, err)

	select {
	case req := <-jenkins.requests:
		require.Equal(t, "", req.Folder)
		require.Equal(t, "orders-build", req.Job)
		require.Equal(t, "feature/orders", req.GitRefValue)
		require.Empty(t, req.ImageTag)
		require.Empty(t, req.ImageRepository)
		require.Zero(t, req.Timeout)
	case <-time.After(time.Second):
		t.Fatal("ordinary build request was not sent to Jenkins")
	}
}

func TestTriggerAutomatedBuildPassesImageRepositoryOverride(t *testing.T) {
	jenkins := &requestCapturingJenkins{requests: make(chan port.BuildRequest, 1)}
	handler := newTriggerHandler(jenkins)

	_, err := handler.TriggerAutomatedBuild(
		context.Background(),
		kernel.NewProjectID(),
		"release",
		"harbor.example/team-a/orders",
		"",
		"",
		nil,
	)
	require.NoError(t, err)

	select {
	case req := <-jenkins.requests:
		require.Equal(t, "release", req.GitRefValue)
		require.Equal(t, "team-a", req.Folder)
		require.Equal(t, "orders", req.Job)
		require.Equal(t, "team-a", req.Namespace)
		require.Equal(t, "orders", req.ProjectName)
		require.Equal(t, "harbor.example/team-a/orders", req.ImageRepository)
	case <-time.After(time.Second):
		t.Fatal("automated build request was not sent to Jenkins")
	}
}

func TestTriggerDeploymentPreservesDeploymentJobAndImageParameters(t *testing.T) {
	jenkins := &requestCapturingJenkins{requests: make(chan port.BuildRequest, 1)}
	handler := newTriggerHandler(jenkins)
	request := port.BuildRequest{
		Namespace:       "team-a",
		ProjectName:     "orders",
		Folder:          "team-a",
		Job:             "orders",
		GitRefValue:     "release/2026-08",
		ImageTag:        "deploy-123",
		ImageRepository: "registry.example/team-a/orders",
	}

	_, err := handler.TriggerDeployment(
		context.Background(),
		kernel.NewWorkspaceID(),
		kernel.NewMemberID(),
		kernel.NewProjectID(),
		request,
	)
	require.NoError(t, err)

	select {
	case got := <-jenkins.requests:
		require.Equal(t, request, got)
	case <-time.After(time.Second):
		t.Fatal("deployment build request was not sent to Jenkins")
	}
}

func TestTriggerDeploymentRejectsMissingOrMismatchedCoordinatesBeforeSave(t *testing.T) {
	valid := port.BuildRequest{
		Namespace:       "team-a",
		ProjectName:     "orders",
		Folder:          "team-a",
		Job:             "orders",
		GitRefValue:     "main",
		ImageTag:        "deploy-123",
		ImageRepository: "registry.example/team-a/orders",
	}
	cases := []struct {
		name   string
		mutate func(*port.BuildRequest)
	}{
		{"missing namespace", func(r *port.BuildRequest) { r.Namespace = "" }},
		{"missing project name", func(r *port.BuildRequest) { r.ProjectName = "" }},
		{"missing folder", func(r *port.BuildRequest) { r.Folder = "" }},
		{"missing job", func(r *port.BuildRequest) { r.Job = "" }},
		{"missing branch", func(r *port.BuildRequest) { r.GitRefValue = "" }},
		{"missing image tag", func(r *port.BuildRequest) { r.ImageTag = "" }},
		{"missing image repository", func(r *port.BuildRequest) { r.ImageRepository = "" }},
		{"folder differs from namespace", func(r *port.BuildRequest) { r.Folder = "other-team" }},
		{"job differs from project", func(r *port.BuildRequest) { r.Job = "other-job" }},
		{"project differs from stored project", func(r *port.BuildRequest) {
			r.ProjectName, r.Job = "other-project", "other-project"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &triggerBuildRepo{}
			jenkins := &requestCapturingJenkins{requests: make(chan port.BuildRequest, 1)}
			handler := newTriggerHandlerWithRepo(repo, jenkins)
			request := valid
			tc.mutate(&request)

			_, err := handler.TriggerDeployment(
				context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), kernel.NewProjectID(), request)

			require.ErrorIs(t, err, domain.ErrInvalidDeploymentBuildRequest)
			require.Zero(t, repo.saves)
			select {
			case <-jenkins.requests:
				t.Fatal("invalid deployment called Jenkins executor")
			case <-time.After(20 * time.Millisecond):
			}
		})
	}
}

func TestTriggerDeploymentNormalizesAndPersistsActualJenkinsPath(t *testing.T) {
	repo := &triggerBuildRepo{}
	jenkins := &requestCapturingJenkins{requests: make(chan port.BuildRequest, 1)}
	handler := newTriggerHandlerWithRepo(repo, jenkins)

	build, err := handler.TriggerDeployment(
		context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), kernel.NewProjectID(),
		port.BuildRequest{
			Namespace: " team-a ", ProjectName: " orders ", Folder: " team-a ", Job: " orders ",
			GitRefValue: " main ", ImageTag: " deploy-123 ",
			ImageRepository: " registry.example/team-a/orders ",
		})
	require.NoError(t, err)

	select {
	case request := <-jenkins.requests:
		require.Equal(t, "team-a", request.Folder)
		require.Equal(t, "orders", request.Job)
		require.Equal(t, "team-a/orders", build.JenkinsJob)
		require.Equal(t, build.JenkinsJob, request.Folder+"/"+request.Job)
	case <-time.After(time.Second):
		t.Fatal("deployment build request was not sent to Jenkins")
	}
}

func TestTriggerDeploymentRejectsNamespaceDifferentFromPersistedProjectBeforeSave(t *testing.T) {
	repo := &triggerBuildRepo{}
	jenkins := &requestCapturingJenkins{requests: make(chan port.BuildRequest, 1)}
	handler := newTriggerHandlerWithRepo(repo, jenkins)

	_, err := handler.TriggerDeployment(
		context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), kernel.NewProjectID(),
		port.BuildRequest{
			Namespace: "client-team", ProjectName: "orders",
			Folder: "client-team", Job: "orders", GitRefValue: "main",
			ImageTag: "deploy-123", ImageRepository: "registry.example/client-team/orders",
		})

	require.ErrorIs(t, err, domain.ErrInvalidDeploymentBuildRequest)
	require.Zero(t, repo.saves)
	select {
	case <-jenkins.requests:
		t.Fatal("namespace mismatch called Jenkins executor")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestTriggerDeploymentAssociatesSavedBuildBeforeStartingExecutor(t *testing.T) {
	repo := &triggerBuildRepo{}
	jenkins := &requestCapturingJenkins{requests: make(chan port.BuildRequest, 1)}
	handler := newTriggerHandlerWithRepo(repo, jenkins)
	associationErr := errors.New("associate run failed")
	callbackSawSaved := false

	_, err := handler.TriggerDeploymentAssociated(
		context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), kernel.NewProjectID(),
		port.BuildRequest{
			Namespace: "team-a", ProjectName: "orders", Folder: "team-a", Job: "orders",
			GitRefValue: "main", ImageTag: "deploy-123", ImageRepository: "registry/orders",
		},
		func(_ context.Context, build *model.Build) error {
			callbackSawSaved = repo.saved == build
			return associationErr
		},
	)

	require.ErrorIs(t, err, associationErr)
	require.True(t, callbackSawSaved)
	require.Equal(t, model.BuildStatusFailed, repo.saved.Status)
	require.Equal(t, 1, repo.updates)
	select {
	case <-jenkins.requests:
		t.Fatal("Jenkins started before successful deployment association")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestTriggerDeploymentCleanupFailureReturnsOnlyInfrastructureError(t *testing.T) {
	cleanupErr := errors.New("build database unavailable")
	associationErr := errors.New("association obsolete")
	repo := &triggerBuildRepo{updateErr: cleanupErr}
	jenkins := &requestCapturingJenkins{requests: make(chan port.BuildRequest, 1)}
	handler := newTriggerHandlerWithRepo(repo, jenkins)

	_, err := handler.TriggerDeploymentAssociated(
		context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), kernel.NewProjectID(),
		port.BuildRequest{
			Namespace: "team-a", ProjectName: "orders", Folder: "team-a", Job: "orders",
			GitRefValue: "main", ImageTag: "tag", ImageRepository: "registry/orders",
		},
		func(context.Context, *model.Build) error { return associationErr },
	)

	require.ErrorIs(t, err, cleanupErr)
	require.NotErrorIs(t, err, associationErr)
}

type countingEventBus struct {
	published int
	err       error
}

func (b *countingEventBus) Publish(context.Context, ...events.Event) error {
	b.published++
	return b.err
}
func (*countingEventBus) Subscribe(string, string, eventbus.Handler) error { return nil }

func TestExecuteBuildReportsTerminalUpdateFailureWithoutPublishing(t *testing.T) {
	updateErr := errors.New("build update unavailable")
	repo := &triggerBuildRepo{updateErr: updateErr, failAfter: 1}
	capturing := &requestCapturingJenkins{requests: make(chan port.BuildRequest, 1)}
	bus := &countingEventBus{}
	asyncErr := make(chan error, 1)
	handler := NewTriggerBuildHandler(
		repo, triggerProjectChecker{namespace: "team-a"}, jenkinsinfra.NewStaticProvider(capturing), bus,
		stream.NewHub(), runner.NewRegistry(), context.Background(), postgres.NewDirectTransactor(), func(err error) { asyncErr <- err },
	)

	build, err := handler.TriggerDeploymentAssociated(
		context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), kernel.NewProjectID(),
		port.BuildRequest{
			Namespace: "team-a", ProjectName: "orders", Folder: "team-a", Job: "orders",
			GitRefValue: "main", ImageTag: "tag", ImageRepository: "registry/orders",
		},
		func(context.Context, *model.Build) error { return nil },
	)
	require.NoError(t, err)
	require.NotNil(t, build.ExecutionStartedAt)
	<-capturing.requests

	select {
	case got := <-asyncErr:
		require.ErrorIs(t, got, updateErr)
	case <-time.After(time.Second):
		t.Fatal("terminal update failure was not observable")
	}
	require.Equal(t, 2, repo.updates)
	require.Zero(t, bus.published)
}

func TestBuildCompletedFinalPublishFailureIsReported(t *testing.T) {
	publishErr := errors.New("event bus unavailable")
	repo := &triggerBuildRepo{}
	capturing := &requestCapturingJenkins{requests: make(chan port.BuildRequest, 1)}
	asyncErr := make(chan error, 1)
	handler := NewTriggerBuildHandler(
		repo, triggerProjectChecker{namespace: "team-a"}, jenkinsinfra.NewStaticProvider(capturing), &countingEventBus{err: publishErr},
		stream.NewHub(), runner.NewRegistry(), context.Background(), postgres.NewDirectTransactor(), func(err error) { asyncErr <- err },
	)
	_, err := handler.Handle(context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(), kernel.NewProjectID(), "main")
	require.NoError(t, err)
	<-capturing.requests
	select {
	case got := <-asyncErr:
		require.ErrorIs(t, got, publishErr)
	case <-time.After(time.Second):
		t.Fatal("final publish failure was not reported")
	}
}

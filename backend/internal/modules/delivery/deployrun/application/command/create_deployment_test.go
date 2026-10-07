package command

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	deliverydomain "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type fakeRunRepo struct {
	active      bool
	created     []*model.DeploymentRun
	updated     []*model.DeploymentRun
	updateErr   error
	found       *model.DeploymentRun
	afterUpdate *model.DeploymentRun
	byProject   []*model.DeploymentRun
}

func (r *fakeRunRepo) Create(_ context.Context, run *model.DeploymentRun) error {
	r.created = append(r.created, run)
	return nil
}
func (r *fakeRunRepo) Update(_ context.Context, run *model.DeploymentRun) error {
	r.updated = append(r.updated, run)
	if r.afterUpdate != nil {
		r.found = r.afterUpdate
	}
	return r.updateErr
}
func (r *fakeRunRepo) FindByID(context.Context, kernel.DeploymentRunID) (*model.DeploymentRun, error) {
	if r.found != nil {
		return r.found, nil
	}
	return nil, domain.ErrDeploymentRunNotFound
}
func (*fakeRunRepo) FindByBuildID(context.Context, kernel.BuildID) (*model.DeploymentRun, error) {
	return nil, domain.ErrDeploymentRunNotFound
}
func (r *fakeRunRepo) ListByProject(context.Context, kernel.ProjectID, int) ([]*model.DeploymentRun, error) {
	return r.byProject, nil
}
func (*fakeRunRepo) ListActive(context.Context) ([]*model.DeploymentRun, error) { return nil, nil }
func (*fakeRunRepo) TryAcquireLease(context.Context, kernel.DeploymentRunID, int64, string, time.Duration) (bool, error) {
	return true, nil
}
func (*fakeRunRepo) ReleaseLease(context.Context, kernel.DeploymentRunID, int64, string) error {
	return nil
}
func (*fakeRunRepo) RecordInfrastructureFailure(context.Context, kernel.DeploymentRunID, int64, string) (*model.DeploymentRun, error) {
	return nil, nil
}
func (*fakeRunRepo) ListEvents(context.Context, kernel.DeploymentRunID) ([]model.DeploymentRunEvent, error) {
	return nil, nil
}
func (*fakeRunRepo) AppendEvent(context.Context, *model.DeploymentRun, string) error {
	return nil
}
func (r *fakeRunRepo) HasActiveByProject(context.Context, kernel.ProjectID) (bool, error) {
	return r.active, nil
}

type fakeDeploymentReader struct {
	result   port.ProjectDeploymentContext
	clusters []port.DeploymentCluster
	loadErr  error
	listErr  error
}

func (r fakeDeploymentReader) Load(context.Context, kernel.WorkspaceID, kernel.ProjectID, kernel.ClusterInfoID) (port.ProjectDeploymentContext, error) {
	return r.result, r.loadErr
}
func (r fakeDeploymentReader) ListAuthorizedClusters(context.Context, kernel.WorkspaceID) ([]port.DeploymentCluster, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	if len(r.clusters) > 0 {
		return r.clusters, nil
	}
	return []port.DeploymentCluster{r.result.Cluster}, nil
}

type recordingQueue struct {
	payloads [][]byte
	err      error
}

func (q *recordingQueue) Publish(_ context.Context, _ string, payload []byte) error {
	q.payloads = append(q.payloads, payload)
	return q.err
}
func (*recordingQueue) Subscribe(context.Context, string, func(context.Context, []byte) error) error {
	return nil
}
func (*recordingQueue) Close() error { return nil }

type recordingBuildAborter struct {
	calls int
	err   error
}

func (a *recordingBuildAborter) Handle(context.Context, kernel.WorkspaceID, kernel.BuildID) error {
	a.calls++
	return a.err
}

type recordingRunCanceler struct {
	calls int
}

func (c *recordingRunCanceler) Cancel(string) bool {
	c.calls++
	return true
}

func approvedReader(wsID kernel.WorkspaceID, projectID kernel.ProjectID, clusterID kernel.ClusterInfoID) fakeDeploymentReader {
	return fakeDeploymentReader{result: port.ProjectDeploymentContext{
		WorkspaceID: wsID, ProjectID: projectID, Cluster: port.DeploymentCluster{ID: clusterID},
	}}
}

func TestCreateDeploymentRejectsActiveRunWithoutPublishing(t *testing.T) {
	wsID, projectID, clusterID := kernel.NewWorkspaceID(), kernel.NewProjectID(), kernel.NewClusterInfoID()
	repo, queue := &fakeRunRepo{active: true}, &recordingQueue{}
	handler := NewCreateDeploymentHandler(repo, approvedReader(wsID, projectID, clusterID), queue, time.Now)

	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, CreateInput{
		GitRefType: "branch", GitRefValue: "main",
	})

	require.ErrorIs(t, err, domain.ErrDeploymentAlreadyRunning)
	require.Empty(t, repo.created)
	require.Empty(t, queue.payloads)
}

func TestCreateInputAcceptsSnakeCaseHTTPFields(t *testing.T) {
	var input CreateInput
	require.NoError(t, json.Unmarshal([]byte(`{
		"cluster_id":"cluster","git_ref_type":"branch","git_ref_value":"main","image_tag":"release","cross_cluster":true
	}`), &input))
	require.Equal(t, "cluster", input.ClusterID)
	require.Equal(t, "branch", input.GitRefType)
	require.Equal(t, "main", input.GitRefValue)
	require.Equal(t, "release", input.ImageTag)
	require.True(t, input.CrossCluster)
}

func TestCreateCrossClusterRequiresSyncTargets(t *testing.T) {
	wsID, projectID, clusterID := kernel.NewWorkspaceID(), kernel.NewProjectID(), kernel.NewClusterInfoID()
	repo, queue := &fakeRunRepo{}, &recordingQueue{}
	reader := approvedReader(wsID, projectID, clusterID)
	handler := NewCreateDeploymentHandler(repo, reader, queue, time.Now)

	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, CreateInput{
		GitRefType: "branch", GitRefValue: "main", CrossCluster: true,
	})

	require.ErrorIs(t, err, domain.ErrCrossClusterSyncTargetsMissing)
	require.Empty(t, repo.created)
	require.Empty(t, queue.payloads)
}

func TestCreateCrossClusterForcesBuildCluster(t *testing.T) {
	wsID, projectID := kernel.NewWorkspaceID(), kernel.NewProjectID()
	buildID, syncID := kernel.NewClusterInfoID(), kernel.NewClusterInfoID()
	repo, queue := &fakeRunRepo{}, &recordingQueue{}
	reader := fakeDeploymentReader{
		result: port.ProjectDeploymentContext{
			WorkspaceID: wsID, ProjectID: projectID,
			Cluster: port.DeploymentCluster{ID: buildID, Name: "build"},
		},
		clusters: []port.DeploymentCluster{
			{ID: buildID, Name: "build", IsBuildEnv: true},
			{ID: syncID, Name: "sync"},
		},
	}
	handler := NewCreateDeploymentHandler(repo, reader, queue, time.Now)

	run, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, CreateInput{
		ClusterID: syncID.String(), GitRefType: "branch", GitRefValue: "main", CrossCluster: true,
	})

	require.NoError(t, err)
	require.True(t, run.CrossCluster)
	require.Equal(t, buildID, run.ClusterID)
	require.Len(t, queue.payloads, 1)
}

func TestCreateSyncOnlyReusesSucceededImage(t *testing.T) {
	wsID, projectID := kernel.NewWorkspaceID(), kernel.NewProjectID()
	buildID, syncID := kernel.NewClusterInfoID(), kernel.NewClusterInfoID()
	source := model.NewDeploymentRun(wsID, projectID, buildID, kernel.NewMemberID(), "branch", "main", "v1.2.3")
	source.Status = model.StatusSucceeded
	repo, queue := &fakeRunRepo{byProject: []*model.DeploymentRun{source}}, &recordingQueue{}
	reader := fakeDeploymentReader{
		result: port.ProjectDeploymentContext{
			WorkspaceID: wsID, ProjectID: projectID, Namespace: "demo",
			Cluster: port.DeploymentCluster{ID: buildID, Name: "build"},
		},
		clusters: []port.DeploymentCluster{
			{ID: buildID, Name: "build", IsBuildEnv: true},
			{ID: syncID, Name: "sync"},
		},
	}
	handler := NewCreateDeploymentHandler(repo, reader, queue, time.Now)

	run, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, CreateInput{
		SyncOnly: true,
		Manifests: []model.SyncManifest{{
			Kind: model.SyncManifestKindDeployment,
			Name: "demo",
			YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: demo\n  namespace: demo\n",
		}},
	})

	require.NoError(t, err)
	require.True(t, run.IsSyncOnly())
	require.True(t, run.CrossCluster)
	require.True(t, run.SkipsBuild())
	require.Equal(t, "v1.2.3", run.ImageTag)
	require.Equal(t, "main", run.GitRefValue)
	require.Equal(t, buildID, run.ClusterID)
	require.Len(t, run.SyncManifests, 1)
	require.Len(t, queue.payloads, 1)
}

func TestCreateSyncOnlyRequiresSucceededSource(t *testing.T) {
	wsID, projectID := kernel.NewWorkspaceID(), kernel.NewProjectID()
	buildID, syncID := kernel.NewClusterInfoID(), kernel.NewClusterInfoID()
	repo, queue := &fakeRunRepo{}, &recordingQueue{}
	reader := fakeDeploymentReader{
		result: port.ProjectDeploymentContext{
			WorkspaceID: wsID, ProjectID: projectID,
			Cluster: port.DeploymentCluster{ID: buildID, Name: "build"},
		},
		clusters: []port.DeploymentCluster{
			{ID: buildID, Name: "build", IsBuildEnv: true},
			{ID: syncID, Name: "sync"},
		},
	}
	handler := NewCreateDeploymentHandler(repo, reader, queue, time.Now)

	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, CreateInput{
		SyncOnly: true,
		Manifests: []model.SyncManifest{{
			Kind: model.SyncManifestKindConfigMap,
			Name: "cfg",
			YAML: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cfg\n",
		}},
	})

	require.ErrorIs(t, err, domain.ErrDeploymentNotSyncable)
	require.Empty(t, repo.created)
	require.Empty(t, queue.payloads)
}

func TestCreateSyncOnlyRequiresManifests(t *testing.T) {
	wsID, projectID := kernel.NewWorkspaceID(), kernel.NewProjectID()
	buildID, syncID := kernel.NewClusterInfoID(), kernel.NewClusterInfoID()
	source := model.NewDeploymentRun(wsID, projectID, buildID, kernel.NewMemberID(), "branch", "main", "v1")
	source.Status = model.StatusSucceeded
	repo, queue := &fakeRunRepo{byProject: []*model.DeploymentRun{source}}, &recordingQueue{}
	reader := fakeDeploymentReader{
		result: port.ProjectDeploymentContext{
			WorkspaceID: wsID, ProjectID: projectID, Namespace: "demo",
			Cluster: port.DeploymentCluster{ID: buildID, Name: "build"},
		},
		clusters: []port.DeploymentCluster{
			{ID: buildID, Name: "build", IsBuildEnv: true},
			{ID: syncID, Name: "sync"},
		},
	}
	handler := NewCreateDeploymentHandler(repo, reader, queue, time.Now)

	_, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, CreateInput{
		SyncOnly: true,
	})

	require.ErrorIs(t, err, domain.ErrDeploymentSyncManifestsEmpty)
	require.Empty(t, repo.created)
	require.Empty(t, queue.payloads)
}

func TestCreateDeploymentGateErrorDoesNotCreateRun(t *testing.T) {
	repo, queue := &fakeRunRepo{}, &recordingQueue{}
	reader := fakeDeploymentReader{loadErr: domain.ErrProjectNotApproved}
	handler := NewCreateDeploymentHandler(repo, reader, queue, time.Now)

	_, err := handler.Handle(context.Background(), kernel.NewWorkspaceID(), kernel.NewMemberID(),
		kernel.NewProjectID(), CreateInput{GitRefType: "branch", GitRefValue: "main"})

	require.ErrorIs(t, err, domain.ErrProjectNotApproved)
	require.Empty(t, repo.created)
	require.Empty(t, queue.payloads)
}

func TestCreateDeploymentDefaultsImageTagAndPublishes(t *testing.T) {
	wsID, projectID, clusterID := kernel.NewWorkspaceID(), kernel.NewProjectID(), kernel.NewClusterInfoID()
	repo, queue := &fakeRunRepo{}, &recordingQueue{}
	now := time.Date(2026, 8, 29, 3, 4, 5, 0, time.UTC)
	handler := NewCreateDeploymentHandler(repo, approvedReader(wsID, projectID, clusterID), queue, func() time.Time { return now })

	run, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, CreateInput{
		GitRefType: "branch", GitRefValue: "main",
	})

	require.NoError(t, err)
	require.Equal(t, "main", run.ImageTag)
	require.Equal(t, now, run.CreatedAt)
	require.Len(t, repo.created, 1)
	require.Len(t, queue.payloads, 1)
}

func TestCreateDeploymentAttachesConfigManifestsOnNormalRun(t *testing.T) {
	wsID, projectID, clusterID := kernel.NewWorkspaceID(), kernel.NewProjectID(), kernel.NewClusterInfoID()
	repo, queue := &fakeRunRepo{}, &recordingQueue{}
	handler := NewCreateDeploymentHandler(repo, approvedReader(wsID, projectID, clusterID), queue, time.Now)

	run, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, CreateInput{
		GitRefType: "branch", GitRefValue: "main", ImageTag: "main",
		Manifests: []model.SyncManifest{{
			Kind: model.SyncManifestKindConfigMap,
			Name: "app-config",
			YAML: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app-config\n",
		}},
	})

	require.NoError(t, err)
	require.False(t, run.IsSyncOnly())
	require.Len(t, run.SyncManifests, 1)
	require.Equal(t, "app-config", run.SyncManifests[0].Name)
}

func TestCreateDeploymentPublishFailureMarksRunFailed(t *testing.T) {
	wsID, projectID, clusterID := kernel.NewWorkspaceID(), kernel.NewProjectID(), kernel.NewClusterInfoID()
	repo := &fakeRunRepo{}
	publishErr := errors.New("broker unavailable")
	queue := &recordingQueue{err: publishErr}
	handler := NewCreateDeploymentHandler(repo, approvedReader(wsID, projectID, clusterID), queue, time.Now)

	run, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, CreateInput{
		GitRefType: "branch", GitRefValue: "main",
	})

	require.ErrorIs(t, err, publishErr)
	require.Nil(t, run)
	require.Len(t, repo.updated, 1)
	require.Equal(t, model.StatusFailed, repo.updated[0].Status)
}

func TestCreateDeploymentReturnsPublishAndPersistenceErrorsWhenFailMarkFails(t *testing.T) {
	wsID, projectID, clusterID := kernel.NewWorkspaceID(), kernel.NewProjectID(), kernel.NewClusterInfoID()
	publishErr := errors.New("broker unavailable")
	persistenceErr := errors.New("database unavailable")
	repo := &fakeRunRepo{updateErr: persistenceErr}
	queue := &recordingQueue{err: publishErr}
	handler := NewCreateDeploymentHandler(repo, approvedReader(wsID, projectID, clusterID), queue, time.Now)

	run, err := handler.Handle(context.Background(), wsID, kernel.NewMemberID(), projectID, CreateInput{
		GitRefType: "branch", GitRefValue: "main",
	})

	require.Nil(t, run)
	require.ErrorIs(t, err, publishErr)
	require.ErrorIs(t, err, persistenceErr)
	require.Contains(t, err.Error(), "mark deployment failed")
}

func TestAbortDeploymentTransitionsOwnedActiveRun(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(),
		kernel.NewMemberID(), "branch", "main", "")
	repo := &fakeRunRepoWithRun{fakeRunRepo: fakeRunRepo{}, run: run}
	now := time.Date(2026, 8, 29, 5, 6, 7, 0, time.UTC)

	aborted, err := NewAbortDeploymentHandler(repo, func() time.Time { return now }).
		Handle(context.Background(), wsID, run.ID)

	require.NoError(t, err)
	require.Equal(t, model.StatusAborted, aborted.Status)
	require.Equal(t, now, *aborted.FinishedAt)
	require.Len(t, repo.updated, 1)
}

func TestAbortDeploymentConcurrentTerminalUpdateReturnsLatestWithoutOverwrite(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")
	latest := *run
	latest.Status = model.StatusSucceeded
	repo := &fakeRunRepo{
		found: run, afterUpdate: &latest,
		updateErr: domain.ErrDeploymentRunConcurrentUpdate,
	}
	handler := NewAbortDeploymentHandler(repo, time.Now)

	result, err := handler.Handle(context.Background(), wsID, run.ID)
	require.NoError(t, err)
	require.Equal(t, model.StatusSucceeded, result.Status)
}

func TestAbortBuildingStopsBuildBeforePersistingRunAbort(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")
	run.Status = model.StatusBuilding
	buildID := kernel.NewBuildID()
	run.BuildID = &buildID
	repo := &fakeRunRepoWithRun{run: run}
	buildAborter := &recordingBuildAborter{}
	handler := NewAbortDeploymentHandler(repo, time.Now, AbortDependencies{Builds: buildAborter})

	aborted, err := handler.Handle(context.Background(), wsID, run.ID)

	require.NoError(t, err)
	require.Equal(t, 1, buildAborter.calls)
	require.Len(t, repo.updated, 1)
	require.Equal(t, model.StatusAborted, aborted.Status)
}

func TestAbortBuildingFailureDoesNotPretendRunWasAborted(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")
	run.Status = model.StatusBuilding
	buildID := kernel.NewBuildID()
	run.BuildID = &buildID
	repo := &fakeRunRepoWithRun{run: run}
	abortErr := errors.New("build cancellation failed")
	handler := NewAbortDeploymentHandler(repo, time.Now, AbortDependencies{
		Builds: &recordingBuildAborter{err: abortErr},
	})

	result, err := handler.Handle(context.Background(), wsID, run.ID)

	require.Nil(t, result)
	require.ErrorIs(t, err, abortErr)
	require.Empty(t, repo.updated)
	require.Equal(t, model.StatusBuilding, run.Status)
}

func TestAbortBuildingStillAbortsRunWhenBuildAlreadyFinished(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")
	run.Status = model.StatusBuilding
	buildID := kernel.NewBuildID()
	run.BuildID = &buildID
	repo := &fakeRunRepoWithRun{run: run}
	handler := NewAbortDeploymentHandler(repo, time.Now, AbortDependencies{
		Builds: &recordingBuildAborter{err: deliverydomain.ErrBuildNotRunning},
	})

	aborted, err := handler.Handle(context.Background(), wsID, run.ID)

	require.NoError(t, err)
	require.Len(t, repo.updated, 1)
	require.Equal(t, model.StatusAborted, aborted.Status)
}

func TestAbortDeployingCancelsOperationAndRepeatedAbortIsIdempotent(t *testing.T) {
	wsID := kernel.NewWorkspaceID()
	run := model.NewDeploymentRun(wsID, kernel.NewProjectID(), kernel.NewClusterInfoID(), kernel.NewMemberID(), "branch", "main", "")
	run.Status = model.StatusDeploying
	repo := &fakeRunRepoWithRun{run: run}
	canceler := &recordingRunCanceler{}
	handler := NewAbortDeploymentHandler(repo, time.Now, AbortDependencies{Runs: canceler})

	first, err := handler.Handle(context.Background(), wsID, run.ID)
	require.NoError(t, err)
	second, err := handler.Handle(context.Background(), wsID, run.ID)

	require.NoError(t, err)
	require.Equal(t, model.StatusAborted, first.Status)
	require.Equal(t, model.StatusAborted, second.Status)
	require.Equal(t, 1, canceler.calls)
	require.Len(t, repo.updated, 1)
}

type fakeRunRepoWithRun struct {
	fakeRunRepo
	run *model.DeploymentRun
}

func (r *fakeRunRepoWithRun) FindByID(context.Context, kernel.DeploymentRunID) (*model.DeploymentRun, error) {
	return r.run, nil
}

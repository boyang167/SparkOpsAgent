package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type pipelineProjectRepo struct {
	project       *model.Project
	finds         int
	transitionErr error
}

func (r *pipelineProjectRepo) BeginArchive(
	_ context.Context, id kernel.ProjectID, generation string, allowUndeploy bool,
) (bool, error) {
	if r.project == nil || r.project.ID != id || r.project.Status != model.ProjectStatusActive {
		return false, nil
	}
	r.project.Status = model.ProjectStatusArchiving
	r.project.ArchiveGeneration = generation
	r.project.ArchivePhase = model.ArchivePhasePending
	r.project.ArchiveAllowUndeploy = &allowUndeploy
	return true, nil
}

func (r *pipelineProjectRepo) Save(context.Context, *model.Project) error { return nil }
func (r *pipelineProjectRepo) Update(_ context.Context, project *model.Project) error {
	r.project = project
	return nil
}
func (r *pipelineProjectRepo) TransitionArchiving(
	_ context.Context, id kernel.ProjectID, generation string, expected model.ArchivePhase, target model.ProjectStatus,
) (bool, error) {
	if r.transitionErr != nil {
		err := r.transitionErr
		r.transitionErr = nil
		return false, err
	}
	if r.project == nil || r.project.ID != id || !r.project.IsArchiving() ||
		r.project.ArchiveGeneration != generation || r.project.ArchivePhase != expected {
		return false, nil
	}
	r.project.Status = target
	r.project.ArchivePhase = model.ArchivePhaseNone
	r.project.ArchiveAllowUndeploy = nil
	if target == model.ProjectStatusActive {
		r.project.ArchiveGeneration = ""
	}
	return true, nil
}
func (r *pipelineProjectRepo) TransitionArchivePhase(
	_ context.Context,
	id kernel.ProjectID,
	generation string,
	expected model.ArchivePhase,
	target model.ArchivePhase,
) (bool, error) {
	if r.project == nil || r.project.ID != id || !r.project.IsArchiving() ||
		r.project.ArchiveGeneration != generation || r.project.ArchivePhase != expected {
		return false, nil
	}
	r.project.ArchivePhase = target
	return true, nil
}
func (r *pipelineProjectRepo) InitializeArchiveGeneration(
	_ context.Context, id kernel.ProjectID, generation string,
) (bool, error) {
	if r.project == nil || r.project.ID != id || !r.project.IsArchiving() ||
		r.project.ArchiveGeneration != "" {
		return false, nil
	}
	r.project.ArchiveGeneration = generation
	r.project.ArchivePhase = model.ArchivePhasePending
	return true, nil
}
func (r *pipelineProjectRepo) FindByID(_ context.Context, id kernel.ProjectID) (*model.Project, error) {
	r.finds++
	if r.project == nil || r.project.ID != id {
		return nil, domain.ErrProjectNotFound
	}
	return r.project, nil
}

func TestArchiveOperationsWithEmptyGenerationCompleteWithoutRepositoryOrProviderCalls(t *testing.T) {
	projectID, workspaceID, memberID := kernel.NewProjectID(), kernel.NewWorkspaceID(), kernel.NewMemberID()
	repo := &pipelineProjectRepo{}
	providerCalls := 0

	undeploy := NewProjectUndeployHandler(repo, func() appctx.ProjectTeardown {
		providerCalls++
		return nil
	}, nil, postgres.NewDirectTransactor())
	require.NoError(t, undeploy(context.Background(), operationWithPayload(t, ProjectUndeployPayload{
		ProjectID: projectID.String(), MemberID: memberID.String(),
	})))

	pipeline := NewPipelineDisableHandler(func() PipelineDisabler {
		providerCalls++
		return nil
	}, repo, postgres.NewDirectTransactor())
	require.NoError(t, pipeline(context.Background(), operationWithPayload(t, PipelineDisablePayload{
		WorkspaceID: workspaceID.String(), ProjectID: projectID.String(),
	})))

	require.Zero(t, repo.finds)
	require.Zero(t, providerCalls)
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
	disabled    bool
	disableErr  error
	calls       int
	statusCalls int
	statusHook  func()
}

func operationWithPayload(t *testing.T, value any) messaging.Operation {
	t.Helper()
	payload, err := json.Marshal(value)
	require.NoError(t, err)
	return messaging.Operation{Payload: payload}
}

func (d *pipelineDisabler) PipelineDisabled(context.Context, kernel.WorkspaceID, kernel.ProjectID) (bool, error) {
	d.statusCalls++
	if d.statusHook != nil {
		d.statusHook()
		d.statusHook = nil
	}
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
	project.ArchivePhase = model.ArchivePhasePipelinePending
	repo := &pipelineProjectRepo{project: project}
	disabler := &pipelineDisabler{disableErr: errors.New("jenkins unavailable")}
	handler := NewPipelineDisableHandler(func() PipelineDisabler { return disabler }, repo, postgres.NewDirectTransactor())
	payload, err := json.Marshal(PipelineDisablePayload{
		WorkspaceID: project.WorkspaceID.String(), ProjectID: project.ID.String(), Generation: project.ArchiveGeneration,
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

func TestPipelineDisableRetryReconcilesExternalSuccessAfterTerminalWriteFailure(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchiving())
	project.ArchivePhase = model.ArchivePhasePipelinePending
	repo := &pipelineProjectRepo{project: project, transitionErr: errors.New("database unavailable")}
	disabler := &pipelineDisabler{}
	handler := NewPipelineDisableHandler(func() PipelineDisabler { return disabler }, repo, postgres.NewDirectTransactor())
	operation := operationWithPayload(t, PipelineDisablePayload{
		WorkspaceID: project.WorkspaceID.String(), ProjectID: project.ID.String(), Generation: project.ArchiveGeneration,
	})

	require.Error(t, handler(context.Background(), operation))
	require.True(t, disabler.disabled)
	require.Equal(t, 1, disabler.calls)
	require.Equal(t, model.ArchivePhaseFinalizing, project.ArchivePhase)

	require.NoError(t, handler(context.Background(), operation))
	require.Equal(t, 1, disabler.calls)
	require.Equal(t, model.ProjectStatusArchived, project.Status)
}

func TestPipelineDisableClaimsFinalizingBeforeExternalQuery(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchiving())
	project.ArchivePhase = model.ArchivePhasePipelinePending
	repo := &pipelineProjectRepo{project: project}
	phaseAtExternal := model.ArchivePhaseNone
	abortWon := false
	disabler := &pipelineDisabler{disabled: true}
	disabler.statusHook = func() {
		phaseAtExternal = project.ArchivePhase
		abortWon, err = repo.TransitionArchiving(
			context.Background(), project.ID, project.ArchiveGeneration,
			model.ArchivePhasePending, model.ProjectStatusActive,
		)
		require.NoError(t, err)
	}
	handler := NewPipelineDisableHandler(func() PipelineDisabler { return disabler }, repo, postgres.NewDirectTransactor())

	require.NoError(t, handler(context.Background(), operationWithPayload(t, PipelineDisablePayload{
		WorkspaceID: project.WorkspaceID.String(), ProjectID: project.ID.String(), Generation: project.ArchiveGeneration,
	})))

	require.Equal(t, model.ArchivePhaseFinalizing, phaseAtExternal)
	require.False(t, abortWon)
	require.Equal(t, model.ProjectStatusArchived, project.Status)
}

func TestPipelineDisableAbortWinnerPreventsExternalQuery(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchiving())
	generation := project.ArchiveGeneration
	repo := &pipelineProjectRepo{project: project}
	aborted, err := repo.TransitionArchiving(
		context.Background(), project.ID, generation, model.ArchivePhasePending, model.ProjectStatusActive,
	)
	require.NoError(t, err)
	require.True(t, aborted)
	disabler := &pipelineDisabler{}
	handler := NewPipelineDisableHandler(func() PipelineDisabler { return disabler }, repo, postgres.NewDirectTransactor())

	require.NoError(t, handler(context.Background(), operationWithPayload(t, PipelineDisablePayload{
		WorkspaceID: project.WorkspaceID.String(), ProjectID: project.ID.String(), Generation: generation,
	})))
	require.Zero(t, disabler.statusCalls)
	require.Zero(t, disabler.calls)
}

type txMarkerKey struct{}

type markerTransactor struct{}

func (markerTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(context.WithValue(ctx, txMarkerKey{}, true))
}

type operationTestClock struct{ now time.Time }

func (c operationTestClock) Now() time.Time { return c.now }

type undeployTeardown struct {
	online       []kernel.ClusterInfoID
	active       bool
	starts       int
	onlineChecks int
	calledInside bool
	lastRequest  appctx.UndeployRequest
	onlineHook   func()
	startHook    func()
}

func (t *undeployTeardown) mark(ctx context.Context) {
	if ctx.Value(txMarkerKey{}) == true {
		t.calledInside = true
	}
}
func (t *undeployTeardown) OnlineClusters(ctx context.Context, _ kernel.WorkspaceID, _ kernel.ProjectID) ([]kernel.ClusterInfoID, error) {
	t.mark(ctx)
	t.onlineChecks++
	if t.onlineHook != nil {
		t.onlineHook()
		t.onlineHook = nil
	}
	return t.online, nil
}
func (t *undeployTeardown) HasActiveRun(ctx context.Context, _ kernel.ProjectID) (bool, error) {
	t.mark(ctx)
	return t.active, nil
}
func (t *undeployTeardown) StartUndeploy(ctx context.Context, req appctx.UndeployRequest) (kernel.DeploymentRunID, error) {
	t.mark(ctx)
	if t.startHook != nil {
		t.startHook()
		t.startHook = nil
	}
	t.starts++
	t.lastRequest = req
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
		AllowUndeploy: boolRef(true), Generation: project.ArchiveGeneration,
	})
	require.NoError(t, err)
	operation := messaging.Operation{
		EventID: "event-1", Owner: "workspace-app.archive-resumer", Payload: payload,
	}

	err = handler(context.Background(), operation)
	require.Error(t, err)
	require.False(t, messaging.IsPermanent(err))
	require.Equal(t, 1, teardown.starts)
	require.Equal(t, project.ArchiveGeneration, teardown.lastRequest.ArchiveGeneration)
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

func TestProjectUndeployClaimWinsConcurrentAbortBeforeExternalQuery(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchiving())
	repo := &pipelineProjectRepo{project: project}
	teardown := &undeployTeardown{online: []kernel.ClusterInfoID{kernel.NewClusterInfoID()}}
	abortWon := false
	phaseAtExternal := model.ArchivePhaseNone
	teardown.onlineHook = func() {
		phaseAtExternal = project.ArchivePhase
	}
	teardown.startHook = func() {
		abortWon, err = repo.TransitionArchiving(
			context.Background(), project.ID, project.ArchiveGeneration,
			model.ArchivePhasePending, model.ProjectStatusActive,
		)
		require.NoError(t, err)
	}
	handler := NewProjectUndeployHandler(
		repo, func() appctx.ProjectTeardown { return teardown }, &undeployOperationWriter{}, postgres.NewDirectTransactor(),
	)

	err = handler(context.Background(), operationWithPayload(t, ProjectUndeployPayload{
		ProjectID: project.ID.String(), MemberID: project.CreatedBy.String(),
		Generation: project.ArchiveGeneration, AllowUndeploy: boolRef(true),
	}))

	require.Error(t, err)
	require.Equal(t, model.ArchivePhaseUndeployClaimed, phaseAtExternal)
	require.False(t, abortWon)
	require.Equal(t, model.ProjectStatusArchiving, project.Status)
	require.Equal(t, 1, teardown.starts)
}

func TestProjectUndeployAbortWinnerPreventsExternalQuery(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchiving())
	generation := project.ArchiveGeneration
	repo := &pipelineProjectRepo{project: project}
	aborted, err := repo.TransitionArchiving(
		context.Background(), project.ID, generation, model.ArchivePhasePending, model.ProjectStatusActive,
	)
	require.NoError(t, err)
	require.True(t, aborted)
	teardown := &undeployTeardown{online: []kernel.ClusterInfoID{kernel.NewClusterInfoID()}}
	handler := NewProjectUndeployHandler(
		repo, func() appctx.ProjectTeardown { return teardown }, &undeployOperationWriter{}, postgres.NewDirectTransactor(),
	)

	require.NoError(t, handler(context.Background(), operationWithPayload(t, ProjectUndeployPayload{
		ProjectID: project.ID.String(), MemberID: project.CreatedBy.String(),
		Generation: generation, AllowUndeploy: boolRef(true),
	})))
	require.Zero(t, teardown.onlineChecks)
	require.Zero(t, teardown.starts)
}

func TestProjectUndeployOldGenerationCannotAffectNewArchiveAttempt(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchiving())
	generationA := project.ArchiveGeneration
	require.NoError(t, project.AbortArchiving())
	require.NoError(t, project.BeginArchiving())
	generationB := project.ArchiveGeneration
	require.NotEqual(t, generationA, generationB)

	repo := &pipelineProjectRepo{project: project}
	teardown := &undeployTeardown{}
	writer := &undeployOperationWriter{}
	handler := NewProjectUndeployHandler(
		repo, func() appctx.ProjectTeardown { return teardown }, writer, markerTransactor{},
	)
	payloadA, err := json.Marshal(ProjectUndeployPayload{
		ProjectID: project.ID.String(), MemberID: project.CreatedBy.String(),
		AllowUndeploy: boolRef(true), Generation: generationA,
	})
	require.NoError(t, err)

	require.NoError(t, handler(context.Background(), messaging.Operation{
		EventID: "event-a", Owner: "workspace-app.archive", Payload: payloadA,
	}))
	require.Zero(t, teardown.onlineChecks)
	require.Zero(t, teardown.starts)
	require.Empty(t, writer.requests)
	require.Equal(t, model.ProjectStatusArchiving, project.Status)
	require.Equal(t, generationB, project.ArchiveGeneration)

	payloadB, err := json.Marshal(ProjectUndeployPayload{
		ProjectID: project.ID.String(), MemberID: project.CreatedBy.String(),
		AllowUndeploy: boolRef(true), Generation: generationB,
	})
	require.NoError(t, err)
	require.NoError(t, handler(context.Background(), messaging.Operation{
		EventID: "event-b", Owner: "workspace-app.archive", Payload: payloadB,
	}))
	require.Equal(t, 1, teardown.onlineChecks)
	require.Len(t, writer.requests, 1)
	require.Equal(t, generationB, writer.requests[0].Payload.(PipelineDisablePayload).Generation)
}

func TestProjectUndeployLegacyRejectRestoresProjectAndCompletes(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchivingWithIntent(false))
	repo := &pipelineProjectRepo{project: project}
	teardown := &undeployTeardown{online: []kernel.ClusterInfoID{kernel.NewClusterInfoID()}}
	writer := &undeployOperationWriter{}
	handler := NewProjectUndeployHandler(
		repo, func() appctx.ProjectTeardown { return teardown }, writer, markerTransactor{},
	)
	payload, err := json.Marshal(ProjectUndeployPayload{
		ProjectID: project.ID.String(), MemberID: project.CreatedBy.String(),
		AllowUndeploy: boolRef(false), Generation: project.ArchiveGeneration,
	})
	require.NoError(t, err)

	err = handler(context.Background(), messaging.Operation{
		EventID: "event-legacy", Owner: "workspace-app.archive", Payload: payload,
	})

	require.NoError(t, err, "nil lets the AMQP dispatcher mark the operation completed")
	require.Equal(t, model.ProjectStatusActive, project.Status)
	require.Zero(t, teardown.starts)
	require.Empty(t, writer.requests)
	require.False(t, teardown.calledInside)
	require.NoError(t, handler(context.Background(), messaging.Operation{
		EventID: "event-legacy", Owner: "workspace-app.archive", Payload: payload,
	}), "completed-operation replay must be idempotent")
}

func TestProjectUndeployLegacyActiveRunAbortsWithoutStartingUndeploy(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchivingWithIntent(false))
	repo := &pipelineProjectRepo{project: project}
	teardown := &undeployTeardown{active: true}
	handler := NewProjectUndeployHandler(
		repo, func() appctx.ProjectTeardown { return teardown }, &undeployOperationWriter{}, postgres.NewDirectTransactor(),
	)

	require.NoError(t, handler(context.Background(), operationWithPayload(t, ProjectUndeployPayload{
		ProjectID: project.ID.String(), MemberID: project.CreatedBy.String(),
		AllowUndeploy: boolRef(false), Generation: project.ArchiveGeneration,
	})))
	require.Equal(t, model.ProjectStatusActive, project.Status)
	require.Zero(t, teardown.starts)
}

func TestProjectUndeployLegacyPipelineAdvanceWinsAbortRace(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchivingWithIntent(false))
	generation := project.ArchiveGeneration
	repo := &pipelineProjectRepo{project: project}
	teardown := &undeployTeardown{online: []kernel.ClusterInfoID{kernel.NewClusterInfoID()}}
	teardown.onlineHook = func() {
		advanced, transitionErr := repo.TransitionArchivePhase(
			context.Background(), project.ID, generation,
			model.ArchivePhasePending, model.ArchivePhasePipelinePending,
		)
		require.NoError(t, transitionErr)
		require.True(t, advanced)
	}
	handler := NewProjectUndeployHandler(
		repo, func() appctx.ProjectTeardown { return teardown }, &undeployOperationWriter{}, postgres.NewDirectTransactor(),
	)

	require.NoError(t, handler(context.Background(), operationWithPayload(t, ProjectUndeployPayload{
		ProjectID: project.ID.String(), MemberID: project.CreatedBy.String(),
		AllowUndeploy: boolRef(false), Generation: generation,
	})))
	require.Equal(t, model.ProjectStatusArchiving, project.Status)
	require.Equal(t, model.ArchivePhasePipelinePending, project.ArchivePhase)
	require.Zero(t, teardown.starts)
}

func TestProjectUndeployLegacyRejectCompletesPersistedAMQPOperation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	now := time.Now().UTC()
	store := messagingpersistence.NewOperationStore(db)
	writer := messaging.NewOperationWriter(store, operationTestClock{now: now})

	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchivingWithIntent(false))
	repo := &pipelineProjectRepo{project: project}
	teardown := &undeployTeardown{online: []kernel.ClusterInfoID{kernel.NewClusterInfoID()}}
	handler := NewProjectUndeployHandler(
		repo, func() appctx.ProjectTeardown { return teardown }, writer, postgres.NewTransactor(db),
	)
	registry := messaging.NewOperationRegistry()
	require.NoError(t, registry.Register(OperationTypeProjectUndeploy, handler))
	eventID := uuid.NewString()
	require.NoError(t, postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		return writer.Schedule(ctx, messaging.OperationRequest{
			Owner: "workspace-app.archive", OperationType: OperationTypeProjectUndeploy,
			IdempotencyKey: eventID + ":workspace-app.archive:" + OperationTypeProjectUndeploy + ":" + project.ID.String() + "/" + project.ArchiveGeneration,
			Payload: ProjectUndeployPayload{
				EventID: eventID, ProjectID: project.ID.String(), MemberID: project.CreatedBy.String(),
				AllowUndeploy: boolRef(false), Generation: project.ArchiveGeneration,
			},
		})
	}))
	var record messagingpersistence.OperationOutbox
	require.NoError(t, db.First(&record).Error)
	require.NoError(t, db.Model(&record).Updates(map[string]any{
		"status": "executing", "locked_by": "test-worker", "locked_at": now, "attempts": 1,
	}).Error)
	operation := messaging.Operation{
		ID: record.ID, EventID: record.EventID, Owner: record.Subscriber,
		OperationType: record.OperationType, IdempotencyKey: record.IdempotencyKey,
		Payload: record.Payload, Attempt: 1,
	}
	registered, ok := registry.Handler(record.OperationType)
	require.True(t, ok)
	require.NoError(t, registered(context.Background(), operation))
	require.NoError(t, store.MarkCompleted(context.Background(), record.ID, "test-worker", now))
	require.Equal(t, model.ProjectStatusActive, project.Status)
	var status string
	require.NoError(t, db.Model(&messagingpersistence.OperationOutbox{}).Select("status").Scan(&status).Error)
	require.Equal(t, "completed", status)
	require.NoError(t, registered(context.Background(), operation))
	require.Zero(t, teardown.starts)
}

func TestProjectUndeployLegacyOfflinePersistsPipelineAndArchives(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	now := time.Now().UTC()
	store := messagingpersistence.NewOperationStore(db)
	writer := messaging.NewOperationWriter(store, operationTestClock{now: now})

	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchivingWithIntent(false))
	repo := &pipelineProjectRepo{project: project}
	teardown := &undeployTeardown{}
	undeploy := NewProjectUndeployHandler(
		repo, func() appctx.ProjectTeardown { return teardown }, writer, postgres.NewTransactor(db),
	)
	eventID := uuid.NewString()
	require.NoError(t, postgres.NewTransactor(db).WithinTransaction(context.Background(), func(ctx context.Context) error {
		return writer.Schedule(ctx, messaging.OperationRequest{
			Owner: "workspace-app.archive", OperationType: OperationTypeProjectUndeploy,
			IdempotencyKey: eventID + ":workspace-app.archive:" + OperationTypeProjectUndeploy + ":" + project.ID.String() + "/" + project.ArchiveGeneration,
			Payload: ProjectUndeployPayload{
				EventID: eventID, ProjectID: project.ID.String(), MemberID: project.CreatedBy.String(),
				AllowUndeploy: boolRef(false), Generation: project.ArchiveGeneration,
			},
		})
	}))
	var undeployRow messagingpersistence.OperationOutbox
	require.NoError(t, db.Where("operation_type = ?", OperationTypeProjectUndeploy).Take(&undeployRow).Error)
	require.NoError(t, undeploy(context.Background(), messaging.Operation{
		ID: undeployRow.ID, EventID: undeployRow.EventID, Owner: undeployRow.Subscriber,
		OperationType: undeployRow.OperationType, Payload: undeployRow.Payload,
	}))
	require.Zero(t, teardown.starts)
	require.Equal(t, model.ArchivePhasePipelinePending, project.ArchivePhase)

	var pipelineRow messagingpersistence.OperationOutbox
	require.NoError(t, db.Where("operation_type = ?", OperationTypePipelineDisable).Take(&pipelineRow).Error)
	disabler := &pipelineDisabler{}
	pipeline := NewPipelineDisableHandler(
		func() PipelineDisabler { return disabler }, repo, postgres.NewTransactor(db),
	)
	require.NoError(t, pipeline(context.Background(), messaging.Operation{
		ID: pipelineRow.ID, EventID: pipelineRow.EventID, Owner: pipelineRow.Subscriber,
		OperationType: pipelineRow.OperationType, Payload: pipelineRow.Payload,
	}))
	require.Equal(t, model.ProjectStatusArchived, project.Status)
	require.Zero(t, teardown.starts)
}

func boolRef(value bool) *bool {
	return &value
}

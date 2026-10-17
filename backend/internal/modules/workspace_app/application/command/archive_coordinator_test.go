package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	workspaceoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type archiveProjectRepo struct {
	projects map[kernel.ProjectID]*model.Project
	updates  int
}

type archiveSQLRepo struct {
	*archiveProjectRepo
	db *gorm.DB
}

func (r *archiveSQLRepo) Save(ctx context.Context, project *model.Project) error {
	r.projects[project.ID] = project
	return postgres.DB(ctx, r.db).Exec(
		"INSERT INTO workspace_app.projects (id, status, archive_generation) VALUES (?, ?, ?)",
		project.ID.String(), string(project.Status), nullableGeneration(project.ArchiveGeneration),
	).Error
}

func (r *archiveSQLRepo) Update(ctx context.Context, project *model.Project) error {
	if err := postgres.DB(ctx, r.db).Exec(
		"UPDATE workspace_app.projects SET status = ?, archive_generation = ? WHERE id = ?",
		string(project.Status), nullableGeneration(project.ArchiveGeneration), project.ID.String(),
	).Error; err != nil {
		return err
	}
	r.projects[project.ID] = project
	return nil
}

func (r *archiveSQLRepo) TransitionArchiving(
	ctx context.Context, id kernel.ProjectID, generation string, target model.ProjectStatus,
) (bool, error) {
	result := postgres.DB(ctx, r.db).Exec(
		"UPDATE workspace_app.projects SET status = ? WHERE id = ? AND status = ? AND archive_generation = ?",
		string(target), id.String(), string(model.ProjectStatusArchiving), generation,
	)
	if result.Error != nil || result.RowsAffected == 0 {
		return false, result.Error
	}
	r.projects[id].Status = target
	return true, nil
}

func (r *archiveSQLRepo) InitializeArchiveGeneration(
	ctx context.Context, id kernel.ProjectID, generation string,
) (bool, error) {
	result := postgres.DB(ctx, r.db).Exec(
		"UPDATE workspace_app.projects SET archive_generation = ? WHERE id = ? AND status = ? AND archive_generation IS NULL",
		generation, id.String(), string(model.ProjectStatusArchiving),
	)
	if result.Error != nil || result.RowsAffected == 0 {
		return false, result.Error
	}
	r.projects[id].ArchiveGeneration = generation
	return true, nil
}

func (r *archiveSQLRepo) FindByID(ctx context.Context, id kernel.ProjectID) (*model.Project, error) {
	var row struct {
		Status            string
		ArchiveGeneration *string
	}
	if err := postgres.DB(ctx, r.db).Table("workspace_app.projects").
		Select("status, archive_generation").Where("id = ?", id.String()).Scan(&row).Error; err != nil {
		return nil, err
	}
	project, ok := r.projects[id]
	if !ok || row.Status == "" {
		return nil, domain.ErrProjectNotFound
	}
	project.Status = model.ProjectStatus(row.Status)
	project.ArchiveGeneration = ""
	if row.ArchiveGeneration != nil {
		project.ArchiveGeneration = *row.ArchiveGeneration
	}
	return project, nil
}

func nullableGeneration(generation string) any {
	if generation == "" {
		return nil
	}
	return generation
}

func (r *archiveSQLRepo) ListByStatus(ctx context.Context, status model.ProjectStatus) ([]*model.Project, error) {
	var ids []string
	if err := postgres.DB(ctx, r.db).Table("workspace_app.projects").
		Where("status = ?", string(status)).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Project, 0, len(ids))
	for _, raw := range ids {
		id, err := kernel.ParseProjectID(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, r.projects[id])
	}
	return out, nil
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
	online        []kernel.ClusterInfoID
	onlineChecks  int
	active        bool
	started       []kernel.ClusterInfoID
	disabled      int
	disableErr    error
	startErr      error
	disabledState bool
	statusChecks  int
}

type archiveOperationWriter struct {
	requests []messaging.OperationRequest
}

func (w *archiveOperationWriter) Schedule(_ context.Context, request messaging.OperationRequest) error {
	w.requests = append(w.requests, request)
	return nil
}

func (f *fakeTeardown) OnlineClusters(context.Context, kernel.WorkspaceID, kernel.ProjectID) ([]kernel.ClusterInfoID, error) {
	f.onlineChecks++
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
	if f.disableErr == nil {
		f.disabledState = true
	}
	return f.disableErr
}
func (f *fakeTeardown) PipelineDisabled(context.Context, kernel.WorkspaceID, kernel.ProjectID) (bool, error) {
	f.statusChecks++
	return f.disabledState, nil
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

func TestArchiveRejectsProjectBoundToProduct(t *testing.T) {
	handler, _, teardown, project := archiveFixture(t)
	productID := kernel.NewProductID()
	project.ProductID = &productID

	_, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders", Teardown: true})

	require.ErrorIs(t, err, domain.ErrProjectBoundToProduct)
	require.Equal(t, model.ProjectStatusActive, project.Status)
	require.Zero(t, teardown.disabled)
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

func TestCoordinatorDoesNotArchiveWhenDisableFails(t *testing.T) {
	_, coordinator, teardown, project := archiveFixture(t)
	require.NoError(t, project.BeginArchiving())
	teardown.disableErr = errors.New("jenkins down")

	err := coordinator.Advance(context.Background(), project, kernel.NewMemberID())

	require.Error(t, err)
	require.Equal(t, model.ProjectStatusArchiving, project.Status)

	teardown.disableErr = nil
	require.NoError(t, coordinator.Advance(context.Background(), project, kernel.NewMemberID()))
	require.Equal(t, model.ProjectStatusArchived, project.Status)
}

func TestCoordinatorSchedulesPipelineDisableBeforeArchive(t *testing.T) {
	_, _, teardown, project := archiveFixture(t)
	require.NoError(t, project.BeginArchiving())
	repo := &archiveProjectRepo{projects: map[kernel.ProjectID]*model.Project{project.ID: project}}
	writer := &archiveOperationWriter{}
	coordinator := command.NewArchiveCoordinator(repo, func() appctx.ProjectTeardown { return teardown }, nil, writer)
	ctx := events.WithMetadata(context.Background(), events.Metadata{EventID: "event-1", Subscriber: "workspace-app.archive"})

	require.NoError(t, coordinator.Advance(ctx, project, kernel.NewMemberID()))

	require.Equal(t, model.ProjectStatusArchiving, project.Status)
	require.Zero(t, teardown.disabled)
	require.Len(t, writer.requests, 1)
	require.Equal(t, workspaceoutbound.OperationTypePipelineDisable, writer.requests[0].OperationType)
	require.Equal(
		t,
		"event-1:workspace-app.archive:workspace-app.pipeline.disable.v1:"+project.ID.String()+"/"+project.ArchiveGeneration,
		writer.requests[0].IdempotencyKey,
	)
}

func TestArchiveCommandSchedulesAdvanceWithoutCallingTeardown(t *testing.T) {
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	repo := &archiveProjectRepo{projects: map[kernel.ProjectID]*model.Project{project.ID: project}}
	teardown := &fakeTeardown{}
	writer := &archiveOperationWriter{}
	coordinator := command.NewArchiveCoordinator(repo, func() appctx.ProjectTeardown { return teardown }, nil, writer)
	handler := command.NewArchiveProjectHandler(repo, updateWorkspaceChecker{}, coordinator, postgres.NewDirectTransactor())

	result, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders"})

	require.NoError(t, err)
	require.Equal(t, string(model.ProjectStatusArchiving), result.Status)
	require.Equal(t, model.ProjectStatusArchiving, project.Status)
	require.Zero(t, teardown.disabled)
	require.Zero(t, teardown.onlineChecks)
	require.Len(t, writer.requests, 1)
	require.Equal(t, workspaceoutbound.OperationTypeProjectUndeploy, writer.requests[0].OperationType)
}

func TestArchiveCommandPersistsPipelineDisableOperation(t *testing.T) {
	db := openArchiveOperationDB(t)
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	repo := &archiveProjectRepo{projects: map[kernel.ProjectID]*model.Project{project.ID: project}}
	teardown := &fakeTeardown{}
	writer := messaging.NewOperationWriter(messagingpersistence.NewOperationStore(db), nil)
	tx := postgres.NewTransactor(db)
	coordinator := command.NewArchiveCoordinator(repo, func() appctx.ProjectTeardown { return teardown }, nil, writer)
	handler := command.NewArchiveProjectHandler(repo, updateWorkspaceChecker{}, coordinator, tx)

	result, err := handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders"})

	require.NoError(t, err)
	require.Equal(t, string(model.ProjectStatusArchiving), result.Status)
	var operationType string
	require.NoError(t, db.Table("platform.operation_outbox").Select("operation_type").Scan(&operationType).Error)
	require.Equal(t, workspaceoutbound.OperationTypeProjectUndeploy, operationType)
}

func TestArchiveCommandCreatesNewOperationForLaterRetry(t *testing.T) {
	db := openArchiveOperationDB(t)
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	repo := &archiveProjectRepo{projects: map[kernel.ProjectID]*model.Project{project.ID: project}}
	teardown := &fakeTeardown{}
	writer := messaging.NewOperationWriter(messagingpersistence.NewOperationStore(db), nil)
	tx := postgres.NewTransactor(db)
	coordinator := command.NewArchiveCoordinator(repo, func() appctx.ProjectTeardown { return teardown }, nil, writer)
	handler := command.NewArchiveProjectHandler(repo, updateWorkspaceChecker{}, coordinator, tx)

	_, err = handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{ConfirmName: "orders"})
	require.NoError(t, err)
	require.NoError(t, project.AbortArchiving())
	_, err = handler.Handle(context.Background(), project.WorkspaceID, kernel.NewMemberID(), project.ID, command.ArchiveInput{
		ConfirmName: "orders", Teardown: true,
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Table("platform.operation_outbox").Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func openArchiveOperationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE platform.operation_outbox (
			id TEXT PRIMARY KEY, event_id TEXT NOT NULL, subscriber TEXT NOT NULL,
			operation_type TEXT NOT NULL, idempotency_key TEXT NOT NULL UNIQUE, payload BLOB NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt_at DATETIME NOT NULL, locked_at DATETIME, locked_by TEXT NOT NULL DEFAULT '',
			completed_at DATETIME, last_error TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL
		)
	`).Error)
	return db
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

func TestResumeAllPersistsStableAdvanceThenPipelineOperationsAndArchivesOnlyAfterExecution(t *testing.T) {
	db := openArchiveResumeDB(t)
	tx := postgres.NewTransactor(db)
	repo := &archiveSQLRepo{
		archiveProjectRepo: &archiveProjectRepo{projects: map[kernel.ProjectID]*model.Project{}},
		db:                 db,
	}
	name, err := model.NewProjectName("orders")
	require.NoError(t, err)
	project := model.NewProject(kernel.NewWorkspaceID(), name, kernel.NewMemberID(), model.CreateProjectInput{Name: "orders"})
	require.NoError(t, project.BeginArchiving())
	require.NoError(t, repo.Save(context.Background(), project))
	clusterID := kernel.NewClusterInfoID()
	teardown := &fakeTeardown{online: []kernel.ClusterInfoID{clusterID}}
	writer := messaging.NewOperationWriter(messagingpersistence.NewOperationStore(db), nil)
	coordinator := command.NewArchiveCoordinator(repo, func() appctx.ProjectTeardown { return teardown }, nil, writer)
	coordinator.UseTransactor(tx)

	require.NoError(t, coordinator.ResumeAll(context.Background()))
	require.NoError(t, coordinator.ResumeAll(context.Background()))
	require.Zero(t, teardown.onlineChecks)

	var rows []struct {
		EventID       string
		Subscriber    string
		OperationType string
		Payload       []byte
	}
	require.NoError(t, db.Table("platform.operation_outbox").Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, "workspace-app.archive-resumer", rows[0].Subscriber)
	require.Equal(t, workspaceoutbound.OperationTypeProjectUndeploy, rows[0].OperationType)
	_, err = uuid.Parse(rows[0].EventID)
	require.NoError(t, err)
	stored, err := repo.FindByID(context.Background(), project.ID)
	require.NoError(t, err)
	require.Equal(t, model.ProjectStatusArchiving, stored.Status)

	advanceHandler := workspaceoutbound.NewProjectUndeployHandler(
		repo, func() appctx.ProjectTeardown { return teardown }, writer, tx,
	)
	operation := messaging.Operation{
		EventID: rows[0].EventID, Owner: rows[0].Subscriber,
		OperationType: rows[0].OperationType, Payload: rows[0].Payload,
	}
	require.Error(t, advanceHandler(context.Background(), operation))
	require.Equal(t, []kernel.ClusterInfoID{clusterID}, teardown.started)
	require.NoError(t, db.Table("platform.operation_outbox").
		Where("operation_type = ?", workspaceoutbound.OperationTypePipelineDisable).
		Find(&rows).Error)
	require.Empty(t, rows)

	// The external completion event is lost; the same operation reconciles offline state on retry.
	teardown.online = nil
	require.NoError(t, advanceHandler(context.Background(), operation))
	require.Equal(t, 2, teardown.onlineChecks)
	require.NoError(t, db.Table("platform.operation_outbox").
		Where("operation_type = ?", workspaceoutbound.OperationTypePipelineDisable).
		Find(&rows).Error)
	require.Len(t, rows, 1)

	pipelineHandler := workspaceoutbound.NewPipelineDisableHandler(
		func() workspaceoutbound.PipelineDisabler { return teardown }, repo, tx,
	)
	require.NoError(t, pipelineHandler(context.Background(), messaging.Operation{
		EventID: rows[0].EventID, Owner: rows[0].Subscriber,
		OperationType: rows[0].OperationType, Payload: rows[0].Payload,
	}))
	stored, err = repo.FindByID(context.Background(), project.ID)
	require.NoError(t, err)
	require.Equal(t, model.ProjectStatusArchived, stored.Status)
}

func openArchiveResumeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS workspace_app").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE workspace_app.projects (id TEXT PRIMARY KEY, status TEXT, archive_generation TEXT);
		CREATE TABLE platform.operation_outbox (
			id TEXT PRIMARY KEY, event_id TEXT NOT NULL, subscriber TEXT NOT NULL,
			operation_type TEXT NOT NULL, idempotency_key TEXT NOT NULL UNIQUE, payload BLOB NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt_at DATETIME NOT NULL, locked_at DATETIME, locked_by TEXT NOT NULL DEFAULT '',
			completed_at DATETIME, last_error TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL
		)
	`).Error)
	return db
}

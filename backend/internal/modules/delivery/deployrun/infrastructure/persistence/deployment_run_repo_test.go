package persistence

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	deployservice "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newRunRepo(t *testing.T) (*DeploymentRunRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// Every new connection to ":memory:" opens an empty database without the attached schemas.
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS delivery").Error)
	require.NoError(t, db.Exec(`CREATE TABLE delivery.deployment_runs (
		id TEXT PRIMARY KEY, workspace_id TEXT, project_id TEXT, cluster_id TEXT,
		git_ref_type TEXT, git_ref_value TEXT, image_tag TEXT, image_repository TEXT NOT NULL DEFAULT '', cross_cluster INTEGER NOT NULL DEFAULT 0,
		sync_manifests TEXT NOT NULL DEFAULT '[]',
		status TEXT, build_id TEXT,
		error_message TEXT, created_by TEXT, created_at DATETIME, started_at DATETIME,
		finished_at DATETIME, current_stage TEXT, run_type TEXT NOT NULL DEFAULT 'normal',
		rollback_of_run_id TEXT, version INTEGER NOT NULL DEFAULT 0,
		attempt_count INTEGER NOT NULL DEFAULT 0, lease_owner TEXT, lease_expires_at DATETIME,
		environment_id TEXT, deploy_namespace TEXT NOT NULL DEFAULT '', project_version_id TEXT, undeploy_options TEXT)`).Error)
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX delivery.uq_deployment_runs_active_project
		ON deployment_runs(project_id) WHERE status IN ('queued','preparing','building','deploying')`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE delivery.deployment_run_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT, at DATETIME, status TEXT, stage TEXT, message TEXT)`).Error)
	return NewDeploymentRunRepository(db), db
}

func TestAutoMigrateFreshSQLiteSchemaSupportsCreateAndList(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS delivery").Error)

	require.NoError(t, AutoMigrate(db))

	repo := NewDeploymentRunRepository(db)
	run := newRun(kernel.NewProjectID())
	require.NoError(t, repo.Create(context.Background(), run))
	runs, err := repo.ListByProject(context.Background(), run.ProjectID, 20)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, run.ID, runs[0].ID)

	for _, column := range []string{"status", "version", "attempt_count", "lease_owner", "lease_expires_at"} {
		var count int64
		require.NoError(t, db.Raw(
			"SELECT count(*) FROM pragma_table_info('deployment_runs', 'delivery') WHERE name=?", column,
		).Scan(&count).Error)
		require.Equalf(t, int64(1), count, "missing deployment_runs.%s", column)
	}
	var eventStatusCount int64
	require.NoError(t, db.Raw(
		"SELECT count(*) FROM pragma_table_info('deployment_run_events', 'delivery') WHERE name='status'",
	).Scan(&eventStatusCount).Error)
	require.Equal(t, int64(1), eventStatusCount)
	for _, index := range []string{
		"idx_deployment_runs_project_created",
		"idx_deployment_runs_status",
		"idx_deployment_run_events_run_at",
	} {
		var count int64
		require.NoError(t, db.Raw(
			"SELECT count(*) FROM delivery.sqlite_master WHERE type='index' AND name=?", index,
		).Scan(&count).Error)
		require.Equalf(t, int64(1), count, "missing index %s", index)
	}
}

type recordingDeploymentEventSink struct {
	events []model.DeploymentRunEvent
}

func (s *recordingDeploymentEventSink) PublishEvent(_ *model.DeploymentRun, event model.DeploymentRunEvent) {
	s.events = append(s.events, event)
}

type panicDeploymentEventSink struct{}

func (panicDeploymentEventSink) PublishEvent(*model.DeploymentRun, model.DeploymentRunEvent) {
	panic("observer included sensitive payload")
}

func newRun(projectID kernel.ProjectID) *model.DeploymentRun {
	run := model.NewDeploymentRun(kernel.NewWorkspaceID(), projectID, kernel.NewClusterInfoID(),
		kernel.NewMemberID(), "branch", "main", "")
	run.CreatedAt = time.Date(2026, 8, 29, 4, 5, 6, 0, time.UTC)
	return run
}

func TestRepositoryCreatePersistsRunAndInitialEvent(t *testing.T) {
	repo, _ := newRunRepo(t)
	run := newRun(kernel.NewProjectID())

	require.NoError(t, repo.Create(context.Background(), run))
	found, err := repo.FindByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, run.ID, found.ID)
	require.Equal(t, run.ImageTag, found.ImageTag)

	events, err := repo.ListEvents(context.Background(), run.ID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "deployment queued", events[0].Message)
	require.Equal(t, model.StatusQueued, events[0].Status)
	require.NotZero(t, events[0].ID)
}

func TestTerminalUpdateAndDeployCompletedOutboxAreAtomic(t *testing.T) {
	repo, db := newRunRepo(t)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	run := newRun(kernel.NewProjectID())
	require.NoError(t, repo.Create(context.Background(), run))
	run.Fail("boom", time.Now().UTC())

	bus := messaging.NewReliableBus(
		sharedevents.NewRegistry(),
		messagingpersistence.NewOutboxStore(db),
		nil, nil, nil,
	)
	repo.SetTerminalCallback(deployservice.NewTerminalCallback(bus))

	require.Error(t, repo.Update(context.Background(), run))
	stored, err := repo.FindByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, model.StatusQueued, stored.Status)
	var outboxCount int64
	require.NoError(t, db.Table("platform.event_outbox").Count(&outboxCount).Error)
	require.Zero(t, outboxCount)
}

func TestTerminalUpdateAndDeployCompletedOutboxCommitTogether(t *testing.T) {
	repo, db := newRunRepo(t)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))

	run := newRun(kernel.NewProjectID())
	environmentID := kernel.NewEnvironmentID()
	run.EnvironmentID = &environmentID
	run.ImageTag = "release-2026.10"
	require.NoError(t, repo.Create(context.Background(), run))
	run.Fail("rollout failed", run.CreatedAt.Add(time.Minute))

	registry := sharedevents.NewRegistry()
	require.NoError(t, deliveryevent.Register(registry))
	bus := messaging.NewReliableBus(
		registry,
		messagingpersistence.NewOutboxStore(db),
		nil, nil, nil,
	)
	repo.SetTerminalCallback(deployservice.NewTerminalCallback(bus))

	require.NoError(t, repo.Update(context.Background(), run))

	stored, err := repo.FindByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, model.StatusFailed, stored.Status)
	deploymentEvents, err := repo.ListEvents(context.Background(), run.ID)
	require.NoError(t, err)
	require.Len(t, deploymentEvents, 2)
	require.Equal(t, model.StatusFailed, deploymentEvents[1].Status)

	var rows []messagingpersistence.EventOutbox
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, deliveryevent.DeployCompletedName, rows[0].EventType)
	require.Equal(t, "deployment_run", rows[0].AggregateType)
	require.Equal(t, run.ID.String(), rows[0].AggregateID)

	var payload deliveryevent.DeployCompleted
	require.NoError(t, json.Unmarshal(rows[0].Payload, &payload))
	require.Equal(t, run.ID, payload.RunID)
	require.Equal(t, run.ProjectID, payload.ProjectID)
	require.Equal(t, run.WorkspaceID, payload.WorkspaceID)
	require.Equal(t, run.EnvironmentID, payload.EnvironmentID)
	require.Empty(t, payload.ProjectName)
	require.Equal(t, string(model.StatusFailed), payload.Status)
	require.Equal(t, string(run.RunType), payload.RunType)
	require.Equal(t, run.ImageTag, payload.ImageTag)
	require.Equal(t, run.CreatedBy, payload.CreatedBy)
}

func TestRepositoryPublishesOnlyCommittedCreateAndUpdateEvents(t *testing.T) {
	_, db := newRunRepo(t)
	sink := &recordingDeploymentEventSink{}
	repo := NewDeploymentRunRepository(db, sink)
	projectID := kernel.NewProjectID()
	run := newRun(projectID)

	require.NoError(t, repo.Create(context.Background(), run))
	require.Equal(t, []model.Status{model.StatusQueued}, eventStatuses(sink.events))

	err := repo.Create(context.Background(), newRun(projectID))
	require.ErrorIs(t, err, domain.ErrDeploymentAlreadyRunning)
	require.Len(t, sink.events, 1, "rolled back writes must not reach the stream sink")

	require.NoError(t, run.Transition(model.StatusPreparing, run.CreatedAt.Add(time.Second)))
	require.NoError(t, repo.Update(context.Background(), run))
	require.Equal(t, []model.Status{model.StatusQueued, model.StatusPreparing}, eventStatuses(sink.events))

	run.Fail("ci failed", run.CreatedAt.Add(2*time.Second))
	require.NoError(t, repo.Update(context.Background(), run))
	require.Equal(t, []model.Status{
		model.StatusQueued, model.StatusPreparing, model.StatusFailed,
	}, eventStatuses(sink.events))
}

func TestRepositoryObserverPanicDoesNotFailCommittedWrite(t *testing.T) {
	_, db := newRunRepo(t)
	var reported error
	repo := NewDeploymentRunRepositoryWithObserver(db, panicDeploymentEventSink{}, func(err error) {
		reported = err
	})
	run := newRun(kernel.NewProjectID())

	require.NoError(t, repo.Create(context.Background(), run))
	_, err := repo.FindByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Error(t, reported)
	require.NotContains(t, reported.Error(), "sensitive payload")
}

func eventStatuses(events []model.DeploymentRunEvent) []model.Status {
	statuses := make([]model.Status, 0, len(events))
	for _, event := range events {
		statuses = append(statuses, event.Status)
	}
	return statuses
}

func TestRepositoryMapsActiveProjectUniqueViolation(t *testing.T) {
	repo, _ := newRunRepo(t)
	projectID := kernel.NewProjectID()
	require.NoError(t, repo.Create(context.Background(), newRun(projectID)))

	err := repo.Create(context.Background(), newRun(projectID))

	require.ErrorIs(t, err, domain.ErrDeploymentAlreadyRunning)
}

func TestRepositoryUpdatePersistsStatusAndEvent(t *testing.T) {
	repo, _ := newRunRepo(t)
	run := newRun(kernel.NewProjectID())
	require.NoError(t, repo.Create(context.Background(), run))
	run.Fail("queue failed", run.CreatedAt.Add(time.Second))

	require.NoError(t, repo.Update(context.Background(), run))

	found, err := repo.FindByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, model.StatusFailed, found.Status)
	require.Equal(t, "queue failed", found.ErrorMessage)
	events, err := repo.ListEvents(context.Background(), run.ID)
	require.NoError(t, err)
	require.Len(t, events, 2)
}

func TestRepositoryFindMissingReturnsDomainError(t *testing.T) {
	repo, _ := newRunRepo(t)
	_, err := repo.FindByID(context.Background(), kernel.NewDeploymentRunID())
	require.ErrorIs(t, err, domain.ErrDeploymentRunNotFound)
}

func TestRepositoryListPageByProjectReturnsTotalAndNewestPage(t *testing.T) {
	repo, _ := newRunRepo(t)
	ctx := context.Background()
	wsID, projectID := kernel.NewWorkspaceID(), kernel.NewProjectID()
	older := newRun(projectID)
	older.WorkspaceID = wsID
	require.NoError(t, repo.Create(ctx, older))
	older.Fail("done", older.CreatedAt)
	require.NoError(t, repo.Update(ctx, older))
	newer := newRun(projectID)
	newer.WorkspaceID = wsID
	newer.CreatedAt = older.CreatedAt.Add(time.Minute)
	require.NoError(t, repo.Create(ctx, newer))

	page, err := repo.ListPageByProject(ctx, wsID, projectID, 1, 1)

	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)
	require.Equal(t, 1, page.Page)
	require.Equal(t, 1, page.PageSize)
	require.Len(t, page.Items, 1)
	require.Equal(t, newer.ID, page.Items[0].ID)
}

func TestRepositoryUpdateRejectsStaleVersionWithoutOverwritingTerminalState(t *testing.T) {
	repo, _ := newRunRepo(t)
	run := newRun(kernel.NewProjectID())
	require.NoError(t, repo.Create(context.Background(), run))
	first, err := repo.FindByID(context.Background(), run.ID)
	require.NoError(t, err)
	stale, err := repo.FindByID(context.Background(), run.ID)
	require.NoError(t, err)

	first.Fail("aborted elsewhere", time.Now())
	require.NoError(t, repo.Update(context.Background(), first))
	stale.Transition(model.StatusPreparing, time.Now())
	err = repo.Update(context.Background(), stale)

	require.ErrorIs(t, err, domain.ErrDeploymentRunConcurrentUpdate)
	found, findErr := repo.FindByID(context.Background(), run.ID)
	require.NoError(t, findErr)
	require.Equal(t, model.StatusFailed, found.Status)
	require.Equal(t, int64(1), found.Version)
}

func TestRepositoryListActiveExcludesTerminalRuns(t *testing.T) {
	repo, _ := newRunRepo(t)
	active := newRun(kernel.NewProjectID())
	terminal := newRun(kernel.NewProjectID())
	require.NoError(t, repo.Create(context.Background(), active))
	require.NoError(t, repo.Create(context.Background(), terminal))
	terminal.Fail("done", time.Now())
	require.NoError(t, repo.Update(context.Background(), terminal))

	runs, err := repo.ListActive(context.Background())

	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, active.ID, runs[0].ID)
}

func TestRepositoryLeaseIsAtomicAndVersioned(t *testing.T) {
	repo, _ := newRunRepo(t)
	run := newRun(kernel.NewProjectID())
	run.Status = model.StatusPreparing
	run.CurrentStage = model.StagePreparing
	require.NoError(t, repo.Create(context.Background(), run))

	acquired, err := repo.TryAcquireLease(context.Background(), run.ID, 0, "worker-a", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
	acquired, err = repo.TryAcquireLease(context.Background(), run.ID, 0, "worker-b", time.Minute)
	require.NoError(t, err)
	require.False(t, acquired)

	found, err := repo.FindByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), found.Version)
	require.Equal(t, "worker-a", found.LeaseOwner)
	require.NotNil(t, found.LeaseExpiresAt)
}

func TestRepositoryLeaseExpiryUsesDatabaseClock(t *testing.T) {
	repo, db := newRunRepo(t)
	run := newRun(kernel.NewProjectID())
	run.Status = model.StatusDeploying
	run.CurrentStage = model.StageDeploying
	require.NoError(t, repo.Create(context.Background(), run))
	acquired, err := repo.TryAcquireLease(context.Background(), run.ID, 0, "worker-a", time.Hour)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NoError(t, db.Exec(
		`UPDATE delivery.deployment_runs SET lease_expires_at=datetime(CURRENT_TIMESTAMP, '-1 second') WHERE id=?`,
		run.ID.String(),
	).Error)

	acquired, err = repo.TryAcquireLease(context.Background(), run.ID, 1, "worker-b", time.Hour)

	require.NoError(t, err)
	require.True(t, acquired)
}

func TestRepositoryListActiveExcludesExhaustedAndLeasedRuns(t *testing.T) {
	repo, db := newRunRepo(t)
	eligible := newRun(kernel.NewProjectID())
	exhausted := newRun(kernel.NewProjectID())
	leased := newRun(kernel.NewProjectID())
	for _, run := range []*model.DeploymentRun{eligible, exhausted, leased} {
		require.NoError(t, repo.Create(context.Background(), run))
	}
	require.NoError(t, db.Exec(`UPDATE delivery.deployment_runs SET attempt_count=3 WHERE id=?`, exhausted.ID.String()).Error)
	require.NoError(t, db.Exec(`UPDATE delivery.deployment_runs SET lease_owner='other', lease_expires_at=? WHERE id=?`,
		time.Now().Add(time.Minute), leased.ID.String()).Error)

	runs, err := repo.ListActive(context.Background())
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, eligible.ID, runs[0].ID)
}

func TestRepositoryRecordInfrastructureFailureUsesSharedThreeAttemptBudget(t *testing.T) {
	repo, _ := newRunRepo(t)
	run := newRun(kernel.NewProjectID())
	run.Status = model.StatusDeploying
	run.CurrentStage = model.StageDeploying
	require.NoError(t, repo.Create(context.Background(), run))

	for attempt := 1; attempt <= 3; attempt++ {
		updated, err := repo.RecordInfrastructureFailure(
			context.Background(), run.ID, run.Version, "broker unavailable",
		)
		require.NoError(t, err)
		run = updated
		require.Equal(t, attempt, run.AttemptCount)
	}
	require.Equal(t, model.StatusFailed, run.Status)
}

func TestRepositoryRecordInfrastructureFailurePublishesEveryCommittedAttempt(t *testing.T) {
	_, db := newRunRepo(t)
	sink := &recordingDeploymentEventSink{}
	repo := NewDeploymentRunRepository(db, sink)
	run := newRun(kernel.NewProjectID())
	run.Status = model.StatusDeploying
	run.CurrentStage = model.StageDeploying
	require.NoError(t, repo.Create(context.Background(), run))
	sink.events = nil

	for attempt := 1; attempt <= 3; attempt++ {
		updated, err := repo.RecordInfrastructureFailure(
			context.Background(), run.ID, run.Version, "broker unavailable",
		)
		require.NoError(t, err)
		run = updated
		require.Len(t, sink.events, attempt)
		event := sink.events[attempt-1]
		require.NotZero(t, event.ID)
		require.Contains(t, event.Message, "infrastructure attempt "+strconv.Itoa(attempt))
		if attempt < 3 {
			require.Equal(t, model.StatusDeploying, event.Status)
		} else {
			require.Equal(t, model.StatusFailed, event.Status)
		}
		if attempt > 1 {
			require.Greater(t, event.ID, sink.events[attempt-2].ID)
		}
	}
}

func TestRepositoryEventInsertFailureRollsBackRunAndDoesNotPublish(t *testing.T) {
	_, db := newRunRepo(t)
	sink := &recordingDeploymentEventSink{}
	repo := NewDeploymentRunRepository(db, sink)
	run := newRun(kernel.NewProjectID())
	run.Status = model.StatusDeploying
	run.CurrentStage = model.StageDeploying
	require.NoError(t, repo.Create(context.Background(), run))
	sink.events = nil
	require.NoError(t, db.Exec("DROP TABLE delivery.deployment_run_events").Error)

	_, err := repo.RecordInfrastructureFailure(
		context.Background(), run.ID, run.Version, "broker unavailable",
	)

	require.Error(t, err)
	require.Empty(t, sink.events)
	found, findErr := repo.FindByID(context.Background(), run.ID)
	require.NoError(t, findErr)
	require.Equal(t, 0, found.AttemptCount)
	require.Equal(t, int64(0), found.Version)
	require.Equal(t, model.StatusDeploying, found.Status)
}

func TestPostgresEventInsertStatementReturnsGeneratedID(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=unused dbname=unused sslmode=disable",
	}), &gorm.Config{
		DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true,
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	row := &deploymentRunEventRow{
		RunID: kernel.NewDeploymentRunID().String(), At: time.Now().UTC(),
		Status: string(model.StatusQueued), Message: "deployment queued",
	}

	result := insertDeploymentEvent(db, row)
	require.NoError(t, result.Error)
	statement := result.Statement.SQL.String()

	require.True(t, strings.Contains(statement, `RETURNING "id"`) ||
		strings.Contains(statement, "RETURNING id"), statement)
}

func TestRepositoryPersistsUndeployOptionsAndQueuedMessage(t *testing.T) {
	repo, _ := newRunRepo(t)
	baseline := newRun(kernel.NewProjectID())
	baseline.Status = model.StatusSucceeded
	run := model.NewUndeployRun(baseline.WorkspaceID, baseline.ProjectID, kernel.NewMemberID(), baseline,
		model.UndeployOptions{DeleteConfig: true, DeleteStorage: true})
	run.CreatedAt = baseline.CreatedAt

	require.NoError(t, repo.Create(context.Background(), run))
	found, err := repo.FindByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, model.RunTypeUndeploy, found.RunType)
	require.Equal(t, &model.UndeployOptions{DeleteConfig: true, DeleteStorage: true}, found.UndeployOptions)

	events, err := repo.ListEvents(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, "undeploy queued", events[0].Message)
}

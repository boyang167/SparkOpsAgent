package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	domainrepo "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type deploymentRunRow struct {
	ID             string `gorm:"column:id;primaryKey"`
	WorkspaceID    string `gorm:"not null"`
	ProjectID      string `gorm:"not null"`
	ClusterID      string `gorm:"not null"`
	GitRefType     string `gorm:"size:16;not null"`
	GitRefValue    string `gorm:"size:255;not null"`
	ImageTag       string `gorm:"size:255;not null"`
	CrossCluster   bool   `gorm:"not null;default:false"`
	SyncManifests  []byte `gorm:"column:sync_manifests;type:jsonb;not null;default:'[]'"`
	Status         string `gorm:"size:16;not null"`
	BuildID        *string
	ErrorMessage   string    `gorm:"not null;default:''"`
	CreatedBy      string    `gorm:"not null"`
	CreatedAt      time.Time `gorm:"not null"`
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CurrentStage      string  `gorm:"size:16;not null;default:''"`
	RunType           string  `gorm:"size:16;not null;default:normal"`
	RollbackOfRunID   *string `gorm:"size:36"`
	Version           int64   `gorm:"not null;default:0"`
	AttemptCount      int     `gorm:"not null;default:0"`
	LeaseOwner        *string
	LeaseExpiresAt    *time.Time
	EnvironmentID     *string
	DeployNamespace   string `gorm:"not null;default:''"`
}

func (deploymentRunRow) TableName() string { return "delivery.deployment_runs" }

type deploymentRunEventRow struct {
	ID      uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	RunID   string    `gorm:"not null"`
	At      time.Time `gorm:"not null"`
	Status  string    `gorm:"size:16;not null"`
	Stage   string    `gorm:"size:16;not null"`
	Message string    `gorm:"not null"`
}

func (deploymentRunEventRow) TableName() string { return "delivery.deployment_run_events" }

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&deploymentRunRow{}, &deploymentRunEventRow{}); err != nil {
		return err
	}
	statements := []string{
		"CREATE INDEX IF NOT EXISTS idx_deployment_runs_project_created ON delivery.deployment_runs(project_id, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_deployment_runs_status ON delivery.deployment_runs(status)",
		"CREATE INDEX IF NOT EXISTS idx_deployment_run_events_run_at ON delivery.deployment_run_events(run_id, at)",
	}
	if db.Dialector.Name() == "sqlite" {
		statements = []string{
			"CREATE INDEX IF NOT EXISTS delivery.idx_deployment_runs_project_created ON deployment_runs(project_id, created_at DESC)",
			"CREATE INDEX IF NOT EXISTS delivery.idx_deployment_runs_status ON deployment_runs(status)",
			"CREATE INDEX IF NOT EXISTS delivery.idx_deployment_run_events_run_at ON deployment_run_events(run_id, at)",
		}
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

type DeploymentRunRepository struct {
	db             *gorm.DB
	sink           DeploymentEventSink
	observerReport func(error)
}

type DeploymentEventSink interface {
	PublishEvent(*model.DeploymentRun, model.DeploymentRunEvent)
}

func NewDeploymentRunRepository(db *gorm.DB, sinks ...DeploymentEventSink) *DeploymentRunRepository {
	repo := &DeploymentRunRepository{db: db}
	if len(sinks) > 0 {
		repo.sink = sinks[0]
	}
	return repo
}

func NewDeploymentRunRepositoryWithObserver(
	db *gorm.DB,
	sink DeploymentEventSink,
	reporter func(error),
) *DeploymentRunRepository {
	return &DeploymentRunRepository{db: db, sink: sink, observerReport: reporter}
}

func (r *DeploymentRunRepository) Create(ctx context.Context, run *model.DeploymentRun) error {
	message := "deployment queued"
	if run.IsRollback() {
		message = "rollback queued"
	}
	eventRow := deploymentRunEventRow{
		RunID: run.ID.String(), At: run.CreatedAt, Status: string(run.Status),
		Stage: string(run.CurrentStage), Message: message,
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(toRunRow(run)).Error; err != nil {
			return err
		}
		return insertDeploymentEvent(tx, &eventRow).Error
	})
	if isActiveRunUniqueViolation(err) {
		return domain.ErrDeploymentAlreadyRunning
	}
	if err == nil {
		r.publishEvent(run, eventRow)
	}
	return err
}

func (r *DeploymentRunRepository) Update(ctx context.Context, run *model.DeploymentRun) error {
	expectedVersion := run.Version
	at := run.CreatedAt
	if run.FinishedAt != nil {
		at = *run.FinishedAt
	} else if run.StartedAt != nil {
		at = *run.StartedAt
	}
	message := "deployment " + string(run.Status)
	if run.IsRollback() {
		message = "rollback " + string(run.Status)
	}
	if run.ErrorMessage != "" {
		message = run.ErrorMessage
	}
	eventRow := deploymentRunEventRow{
		RunID: run.ID.String(), At: at, Status: string(run.Status),
		Stage: string(run.CurrentStage), Message: message,
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&deploymentRunRow{}).
			Where("id = ? AND version = ?", run.ID.String(), expectedVersion).
			Updates(map[string]any{
				"status": string(run.Status), "build_id": buildIDValue(run.BuildID),
				"error_message": run.ErrorMessage, "started_at": run.StartedAt,
				"finished_at": run.FinishedAt, "current_stage": string(run.CurrentStage),
				"version":       expectedVersion + 1,
				"attempt_count": run.AttemptCount, "lease_owner": nullableString(run.LeaseOwner),
				"lease_expires_at": run.LeaseExpiresAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrDeploymentRunConcurrentUpdate
		}
		return insertDeploymentEvent(tx, &eventRow).Error
	})
	if err == nil {
		run.Version = expectedVersion + 1
		r.publishEvent(run, eventRow)
	}
	return err
}

func (r *DeploymentRunRepository) FindByID(ctx context.Context, id kernel.DeploymentRunID) (*model.DeploymentRun, error) {
	var row deploymentRunRow
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrDeploymentRunNotFound
	}
	if err != nil {
		return nil, err
	}
	return fromRunRow(row)
}

func (r *DeploymentRunRepository) FindByBuildID(ctx context.Context, id kernel.BuildID) (*model.DeploymentRun, error) {
	var row deploymentRunRow
	err := r.db.WithContext(ctx).Where("build_id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrDeploymentRunNotFound
	}
	if err != nil {
		return nil, err
	}
	return fromRunRow(row)
}

func (r *DeploymentRunRepository) ListByProject(ctx context.Context, projectID kernel.ProjectID, limit int) ([]*model.DeploymentRun, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []deploymentRunRow
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID.String()).
		Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	runs := make([]*model.DeploymentRun, 0, len(rows))
	for _, row := range rows {
		run, err := fromRunRow(row)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (r *DeploymentRunRepository) ListPageByProject(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	projectID kernel.ProjectID,
	page,
	pageSize int,
) (kernel.Page[*model.DeploymentRun], error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	query := r.db.WithContext(ctx).Model(&deploymentRunRow{}).
		Where("workspace_id = ? AND project_id = ?", wsID.String(), projectID.String())
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return kernel.Page[*model.DeploymentRun]{}, err
	}
	var rows []deploymentRunRow
	if err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return kernel.Page[*model.DeploymentRun]{}, err
	}
	runs := make([]*model.DeploymentRun, 0, len(rows))
	for _, row := range rows {
		run, err := fromRunRow(row)
		if err != nil {
			return kernel.Page[*model.DeploymentRun]{}, err
		}
		runs = append(runs, run)
	}
	return kernel.NewPage(runs, total, page, pageSize), nil
}

func (r *DeploymentRunRepository) ListActive(ctx context.Context) ([]*model.DeploymentRun, error) {
	var rows []deploymentRunRow
	if err := r.db.WithContext(ctx).
		Where("status IN ? AND attempt_count < 3 AND (lease_owner IS NULL OR lease_expires_at IS NULL OR lease_expires_at <= CURRENT_TIMESTAMP)", []string{
			string(model.StatusQueued), string(model.StatusPreparing),
			string(model.StatusBuilding), string(model.StatusDeploying),
		}).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	runs := make([]*model.DeploymentRun, 0, len(rows))
	for _, row := range rows {
		run, err := fromRunRow(row)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (r *DeploymentRunRepository) TryAcquireLease(
	ctx context.Context,
	runID kernel.DeploymentRunID,
	expectedVersion int64,
	owner string,
	ttl time.Duration,
) (bool, error) {
	result := r.db.WithContext(ctx).Model(&deploymentRunRow{}).
		Where(`id = ? AND version = ? AND attempt_count < 3
			AND status IN ? AND (lease_owner IS NULL OR lease_expires_at IS NULL OR lease_expires_at <= CURRENT_TIMESTAMP)`,
			runID.String(), expectedVersion,
			[]string{string(model.StatusPreparing), string(model.StatusDeploying)}).
		Updates(map[string]any{
			"lease_owner": owner, "lease_expires_at": r.leaseExpiryExpr(ttl), "version": expectedVersion + 1,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (r *DeploymentRunRepository) leaseExpiryExpr(ttl time.Duration) any {
	if r.db.Dialector.Name() == "postgres" {
		return gorm.Expr("CURRENT_TIMESTAMP + (? * INTERVAL '1 millisecond')", ttl.Milliseconds())
	}
	return gorm.Expr("datetime(CURRENT_TIMESTAMP, ?)", fmt.Sprintf("+%.6f seconds", ttl.Seconds()))
}

func (r *DeploymentRunRepository) ReleaseLease(
	ctx context.Context,
	runID kernel.DeploymentRunID,
	expectedVersion int64,
	owner string,
) error {
	result := r.db.WithContext(ctx).Model(&deploymentRunRow{}).
		Where("id = ? AND version = ? AND lease_owner = ?", runID.String(), expectedVersion, owner).
		Updates(map[string]any{
			"lease_owner": nil, "lease_expires_at": nil, "version": expectedVersion + 1,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrDeploymentRunConcurrentUpdate
	}
	return nil
}

func (r *DeploymentRunRepository) RecordInfrastructureFailure(
	ctx context.Context,
	runID kernel.DeploymentRunID,
	expectedVersion int64,
	message string,
) (*model.DeploymentRun, error) {
	var row deploymentRunRow
	var eventRow deploymentRunEventRow
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&deploymentRunRow{}).
			Where("id = ? AND version = ? AND status IN ?", runID.String(), expectedVersion, []string{
				string(model.StatusQueued), string(model.StatusPreparing),
				string(model.StatusBuilding), string(model.StatusDeploying),
			}).
			Updates(map[string]any{
				"attempt_count":    gorm.Expr("attempt_count + 1"),
				"status":           gorm.Expr("CASE WHEN attempt_count + 1 >= 3 THEN ? ELSE status END", string(model.StatusFailed)),
				"error_message":    gorm.Expr("CASE WHEN attempt_count + 1 >= 3 THEN ? ELSE error_message END", message),
				"finished_at":      gorm.Expr("CASE WHEN attempt_count + 1 >= 3 THEN CURRENT_TIMESTAMP ELSE finished_at END"),
				"lease_owner":      nil,
				"lease_expires_at": nil,
				"version":          gorm.Expr("version + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrDeploymentRunConcurrentUpdate
		}
		if err := tx.Where("id = ?", runID.String()).First(&row).Error; err != nil {
			return err
		}
		eventRow = deploymentRunEventRow{
			RunID: runID.String(), At: time.Now().UTC(), Status: row.Status, Stage: row.CurrentStage,
			Message: fmt.Sprintf("infrastructure attempt %d: %s", row.AttemptCount, message),
		}
		return insertDeploymentEvent(tx, &eventRow).Error
	})
	if err != nil {
		return nil, err
	}
	run, err := fromRunRow(row)
	if err != nil {
		return nil, err
	}
	r.publishEvent(run, eventRow)
	return run, nil
}

func (r *DeploymentRunRepository) ListEvents(ctx context.Context, runID kernel.DeploymentRunID) ([]model.DeploymentRunEvent, error) {
	var rows []deploymentRunEventRow
	if err := r.db.WithContext(ctx).Where("run_id = ?", runID.String()).Order("at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	events := make([]model.DeploymentRunEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, model.DeploymentRunEvent{
			ID: row.ID, RunID: runID, At: row.At, Status: model.Status(row.Status),
			Stage: model.Stage(row.Stage), Message: row.Message,
		})
	}
	return events, nil
}

func (r *DeploymentRunRepository) AppendEvent(ctx context.Context, run *model.DeploymentRun, message string) error {
	at := time.Now().UTC()
	eventRow := deploymentRunEventRow{
		RunID: run.ID.String(), At: at, Status: string(run.Status),
		Stage: string(run.CurrentStage), Message: message,
	}
	if err := insertDeploymentEvent(r.db.WithContext(ctx), &eventRow).Error; err != nil {
		return err
	}
	r.publishEvent(run, eventRow)
	return nil
}

func (r *DeploymentRunRepository) publishEvent(run *model.DeploymentRun, row deploymentRunEventRow) {
	if r.sink == nil {
		return
	}
	defer func() {
		if recover() != nil {
			r.reportObserverPanic()
		}
	}()
	r.sink.PublishEvent(run, model.DeploymentRunEvent{
		ID: row.ID, RunID: run.ID, At: row.At, Status: model.Status(row.Status),
		Stage: model.Stage(row.Stage), Message: row.Message,
	})
}

func (r *DeploymentRunRepository) reportObserverPanic() {
	if r.observerReport == nil {
		return
	}
	defer func() { _ = recover() }()
	r.observerReport(errors.New("deployment event observer panic"))
}

func insertDeploymentEvent(tx *gorm.DB, row *deploymentRunEventRow) *gorm.DB {
	return tx.Clauses(clause.Returning{
		Columns: []clause.Column{{Name: "id"}},
	}).Create(row)
}

func (r *DeploymentRunRepository) HasActiveByProject(ctx context.Context, projectID kernel.ProjectID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&deploymentRunRow{}).
		Where("project_id = ? AND status IN ?", projectID.String(),
			[]string{string(model.StatusQueued), string(model.StatusPreparing), string(model.StatusBuilding), string(model.StatusDeploying)}).
		Count(&count).Error
	return count > 0, err
}

func toRunRow(run *model.DeploymentRun) *deploymentRunRow {
	runType := string(run.RunType)
	if runType == "" {
		runType = string(model.RunTypeNormal)
	}
	syncManifests, _ := json.Marshal(run.SyncManifests)
	if len(run.SyncManifests) == 0 {
		syncManifests = []byte("[]")
	}
	return &deploymentRunRow{
		ID: run.ID.String(), WorkspaceID: run.WorkspaceID.String(), ProjectID: run.ProjectID.String(),
		ClusterID: run.ClusterID.String(), GitRefType: run.GitRefType, GitRefValue: run.GitRefValue,
		ImageTag: run.ImageTag, CrossCluster: run.CrossCluster, SyncManifests: syncManifests,
		Status: string(run.Status), BuildID: buildIDValue(run.BuildID),
		ErrorMessage: run.ErrorMessage, CreatedBy: run.CreatedBy.String(), CreatedAt: run.CreatedAt,
		StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, CurrentStage: string(run.CurrentStage),
		RunType: runType, RollbackOfRunID: deploymentRunIDValue(run.RollbackOfRunID),
		Version: run.Version,
		AttemptCount: run.AttemptCount, LeaseOwner: nullableString(run.LeaseOwner),
		LeaseExpiresAt: run.LeaseExpiresAt,
		EnvironmentID: environmentIDValue(run.EnvironmentID), DeployNamespace: run.DeployNamespace,
	}
}

func fromRunRow(row deploymentRunRow) (*model.DeploymentRun, error) {
	id, err := kernel.ParseDeploymentRunID(row.ID)
	if err != nil {
		return nil, fmt.Errorf("parse deployment run id: %w", err)
	}
	wsID, err := kernel.ParseWorkspaceID(row.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("parse workspace id: %w", err)
	}
	projectID, err := kernel.ParseProjectID(row.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("parse project id: %w", err)
	}
	clusterID, err := kernel.ParseClusterInfoID(row.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("parse cluster id: %w", err)
	}
	createdBy, err := kernel.ParseMemberID(row.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("parse member id: %w", err)
	}
	runType := model.RunType(row.RunType)
	if runType == "" {
		runType = model.RunTypeNormal
	}
	run := &model.DeploymentRun{
		ID: id, WorkspaceID: wsID, ProjectID: projectID, ClusterID: clusterID,
		GitRefType: row.GitRefType, GitRefValue: row.GitRefValue, ImageTag: row.ImageTag,
		CrossCluster: row.CrossCluster, DeployNamespace: row.DeployNamespace,
		Status: model.Status(row.Status), ErrorMessage: row.ErrorMessage, CreatedBy: createdBy,
		CreatedAt: row.CreatedAt, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt,
		CurrentStage: model.Stage(row.CurrentStage),
		RunType:      runType,
		Version:      row.Version,
		AttemptCount: row.AttemptCount,
		LeaseExpiresAt: row.LeaseExpiresAt,
	}
	if len(row.SyncManifests) > 0 && string(row.SyncManifests) != "null" {
		_ = json.Unmarshal(row.SyncManifests, &run.SyncManifests)
	}
	if row.RollbackOfRunID != nil && *row.RollbackOfRunID != "" {
		rollbackOf, parseErr := kernel.ParseDeploymentRunID(*row.RollbackOfRunID)
		if parseErr != nil {
			return nil, fmt.Errorf("parse rollback_of_run_id: %w", parseErr)
		}
		run.RollbackOfRunID = &rollbackOf
	}
	if row.LeaseOwner != nil {
		run.LeaseOwner = *row.LeaseOwner
	}
	if row.BuildID != nil && *row.BuildID != "" {
		buildID, err := kernel.ParseBuildID(*row.BuildID)
		if err != nil {
			return nil, fmt.Errorf("parse build id: %w", err)
		}
		run.BuildID = &buildID
	}
	if row.EnvironmentID != nil && *row.EnvironmentID != "" {
		envID, err := kernel.ParseEnvironmentID(*row.EnvironmentID)
		if err != nil {
			return nil, fmt.Errorf("parse environment id: %w", err)
		}
		run.EnvironmentID = &envID
	}
	return run, nil
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func buildIDValue(id *kernel.BuildID) *string {
	if id == nil {
		return nil
	}
	value := id.String()
	return &value
}

func deploymentRunIDValue(id *kernel.DeploymentRunID) *string {
	if id == nil {
		return nil
	}
	value := id.String()
	return &value
}

func environmentIDValue(id *kernel.EnvironmentID) *string {
	if id == nil {
		return nil
	}
	value := id.String()
	return &value
}

type sqlStateError interface {
	SQLState() string
}

func isActiveRunUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var state sqlStateError
	if errors.As(err, &state) && state.SQLState() == "23505" &&
		strings.Contains(strings.ToLower(err.Error()), "uq_deployment_runs_active_project") {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") &&
		(strings.Contains(message, "uq_deployment_runs_active_project") ||
			strings.Contains(message, "deployment_runs.project_id"))
}

var _ domainrepo.DeploymentRunRepository = (*DeploymentRunRepository)(nil)

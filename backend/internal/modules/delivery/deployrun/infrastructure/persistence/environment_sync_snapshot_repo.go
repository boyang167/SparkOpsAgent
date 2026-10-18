package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	domainrepo "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type environmentSyncSnapshotRow struct {
	ID                  string     `gorm:"column:id;primaryKey"`
	WorkspaceID         string     `gorm:"not null"`
	ProductID           string     `gorm:"not null"`
	WorkflowID          string     `gorm:"not null"`
	JobName             string     `gorm:"size:255;not null"`
	SourceEnvironmentID string     `gorm:"not null"`
	TargetEnvironmentID string     `gorm:"not null"`
	Projects            []byte     `gorm:"column:projects;type:jsonb;not null"`
	Checksum            string     `gorm:"size:64;not null"`
	CreatedBy           string     `gorm:"not null"`
	BoundRunID          *string    `gorm:"column:bound_run_id"`
	CreatedAt           time.Time  `gorm:"not null"`
	BoundAt             *time.Time `gorm:"column:bound_at"`
}

func (environmentSyncSnapshotRow) TableName() string {
	return "delivery.environment_sync_snapshots"
}

type EnvironmentSyncSnapshotRepository struct {
	db *gorm.DB
}

func NewEnvironmentSyncSnapshotRepository(db *gorm.DB) *EnvironmentSyncSnapshotRepository {
	return &EnvironmentSyncSnapshotRepository{db: db}
}

func (r *EnvironmentSyncSnapshotRepository) Create(ctx context.Context, snapshot *model.EnvironmentSyncSnapshot) error {
	return postgres.DB(ctx, r.db).Create(toSnapshotRow(snapshot)).Error
}

func (r *EnvironmentSyncSnapshotRepository) FindByID(
	ctx context.Context,
	id kernel.EnvironmentSyncSnapshotID,
) (*model.EnvironmentSyncSnapshot, error) {
	var row environmentSyncSnapshotRow
	err := postgres.DB(ctx, r.db).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrEnvironmentSyncSnapshotNotFound
	}
	if err != nil {
		return nil, err
	}
	return fromSnapshotRow(row)
}

func (r *EnvironmentSyncSnapshotRepository) BindToRun(
	ctx context.Context,
	id kernel.EnvironmentSyncSnapshotID,
	runID kernel.ProductWorkflowRunID,
) error {
	result := postgres.DB(ctx, r.db).Model(&environmentSyncSnapshotRow{}).
		Where("id = ? AND bound_run_id IS NULL", id.String()).
		Updates(map[string]any{
			"bound_run_id": runID.String(),
			"bound_at":     gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		existing, err := r.FindByID(ctx, id)
		if err != nil {
			return err
		}
		if existing.BoundRunID != nil {
			return domain.ErrEnvironmentSyncSnapshotAlreadyBound
		}
		return domain.ErrEnvironmentSyncSnapshotNotFound
	}
	return nil
}

func toSnapshotRow(snapshot *model.EnvironmentSyncSnapshot) *environmentSyncSnapshotRow {
	projects, _ := json.Marshal(snapshot.Projects)
	if len(snapshot.Projects) == 0 {
		projects = []byte("[]")
	}
	return &environmentSyncSnapshotRow{
		ID:                  snapshot.ID.String(),
		WorkspaceID:         snapshot.WorkspaceID.String(),
		ProductID:           snapshot.ProductID.String(),
		WorkflowID:          snapshot.WorkflowID.String(),
		JobName:             snapshot.JobName,
		SourceEnvironmentID: snapshot.SourceEnvironmentID.String(),
		TargetEnvironmentID: snapshot.TargetEnvironmentID.String(),
		Projects:            projects,
		Checksum:            snapshot.Checksum,
		CreatedBy:           snapshot.CreatedBy.String(),
		BoundRunID:          productWorkflowRunIDValue(snapshot.BoundRunID),
		CreatedAt:           snapshot.CreatedAt,
		BoundAt:             snapshot.BoundAt,
	}
}

func fromSnapshotRow(row environmentSyncSnapshotRow) (*model.EnvironmentSyncSnapshot, error) {
	id, err := kernel.ParseEnvironmentSyncSnapshotID(row.ID)
	if err != nil {
		return nil, fmt.Errorf("parse environment sync snapshot id: %w", err)
	}
	wsID, err := kernel.ParseWorkspaceID(row.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("parse workspace id: %w", err)
	}
	productID, err := kernel.ParseProductID(row.ProductID)
	if err != nil {
		return nil, fmt.Errorf("parse product id: %w", err)
	}
	workflowID, err := kernel.ParseProductWorkflowID(row.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("parse workflow id: %w", err)
	}
	sourceEnvID, err := kernel.ParseEnvironmentID(row.SourceEnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("parse source environment id: %w", err)
	}
	targetEnvID, err := kernel.ParseEnvironmentID(row.TargetEnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("parse target environment id: %w", err)
	}
	createdBy, err := kernel.ParseMemberID(row.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("parse member id: %w", err)
	}
	snapshot := &model.EnvironmentSyncSnapshot{
		ID:                  id,
		WorkspaceID:         wsID,
		ProductID:           productID,
		WorkflowID:          workflowID,
		JobName:             row.JobName,
		SourceEnvironmentID: sourceEnvID,
		TargetEnvironmentID: targetEnvID,
		Checksum:            row.Checksum,
		CreatedBy:           createdBy,
		CreatedAt:           row.CreatedAt,
		BoundAt:             row.BoundAt,
	}
	if len(row.Projects) > 0 && string(row.Projects) != "null" {
		if err := json.Unmarshal(row.Projects, &snapshot.Projects); err != nil {
			return nil, fmt.Errorf("parse environment sync projects: %w", err)
		}
	}
	if row.BoundRunID != nil && *row.BoundRunID != "" {
		boundRunID, parseErr := kernel.ParseProductWorkflowRunID(*row.BoundRunID)
		if parseErr != nil {
			return nil, fmt.Errorf("parse bound run id: %w", parseErr)
		}
		snapshot.BoundRunID = &boundRunID
	}
	return snapshot, nil
}

func productWorkflowRunIDValue(id *kernel.ProductWorkflowRunID) *string {
	if id == nil {
		return nil
	}
	value := id.String()
	return &value
}

var _ domainrepo.EnvironmentSyncSnapshotRepository = (*EnvironmentSyncSnapshotRepository)(nil)

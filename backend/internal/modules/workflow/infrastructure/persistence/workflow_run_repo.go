package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type workflowRunModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	WorkflowID  string
	MemberID    string
	WorkspaceID string
	Status      string
	Input       string
	Output      string `gorm:"type:text"`
	Error       string
	StartedAt   time.Time
	CompletedAt *time.Time
	postgres.AuditColumns
}

func (workflowRunModel) TableName() string { return "workflow.workflow_runs" }

type WorkflowRunRepository struct{ db *gorm.DB }

func NewWorkflowRunRepository(db *gorm.DB) repository.WorkflowRunRepository {
	return &WorkflowRunRepository{db: db}
}

func (r *WorkflowRunRepository) Save(ctx context.Context, run *model.WorkflowRun) error {
	row := runToRow(run)
	row.AuditColumns = postgres.AuditColumns{CreatedAt: run.StartedAt, UpdatedAt: run.StartedAt}
	return postgres.DB(ctx, r.db).Create(&row).Error
}

func (r *WorkflowRunRepository) Update(ctx context.Context, run *model.WorkflowRun) error {
	return postgres.DB(ctx, r.db).Model(&workflowRunModel{}).Where("id = ?", run.ID.String()).Updates(map[string]any{
		"status":       string(run.Status),
		"output":       run.Output,
		"error":        run.Error,
		"completed_at": run.CompletedAt,
		"updated_at":   time.Now().UTC(),
	}).Error
}

func (r *WorkflowRunRepository) FindByID(ctx context.Context, id kernel.WorkflowRunID) (*model.WorkflowRun, error) {
	var row workflowRunModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrWorkflowRunNotFound
	}
	if err != nil {
		return nil, err
	}
	return runRowToModel(row)
}

func (r *WorkflowRunRepository) ListByWorkflow(ctx context.Context, workflowID kernel.WorkflowID, limit int) ([]*model.WorkflowRun, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []workflowRunModel
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID.String()).
		Order("started_at desc").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.WorkflowRun, 0, len(rows))
	for _, row := range rows {
		run, err := runRowToModel(row)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, nil
}

func runToRow(run *model.WorkflowRun) workflowRunModel {
	return workflowRunModel{
		ID: run.ID.String(), WorkflowID: run.WorkflowID.String(),
		MemberID: run.MemberID.String(), WorkspaceID: run.WorkspaceID.String(),
		Status: string(run.Status), Input: run.Input, Output: run.Output,
		Error: run.Error, StartedAt: run.StartedAt, CompletedAt: run.CompletedAt,
	}
}

func runRowToModel(row workflowRunModel) (*model.WorkflowRun, error) {
	id, _ := kernel.ParseWorkflowRunID(row.ID)
	wfID, _ := kernel.ParseWorkflowID(row.WorkflowID)
	memberID, _ := kernel.ParseMemberID(row.MemberID)
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	return &model.WorkflowRun{
		ID: id, WorkflowID: wfID, MemberID: memberID, WorkspaceID: wsID,
		Status: model.WorkflowRunStatus(row.Status), Input: row.Input,
		Output: row.Output, Error: row.Error,
		StartedAt: row.StartedAt, CompletedAt: row.CompletedAt,
	}, nil
}

var _ repository.WorkflowRunRepository = (*WorkflowRunRepository)(nil)

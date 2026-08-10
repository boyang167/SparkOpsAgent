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

func AutoMigrate(db *gorm.DB) error {
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS workflow").Error; err != nil {
			return err
		}
	}
	return db.AutoMigrate(&workflowModel{}, &workflowRunModel{})
}

type workflowModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	ProjectID   string
	Name        string
	Description string
	Status      string
	Definition  string
	postgres.AuditColumns
}

func (workflowModel) TableName() string { return "workflow.workflows" }

type WorkflowRepository struct{ db *gorm.DB }

func NewWorkflowRepository(db *gorm.DB) repository.WorkflowRepository {
	return &WorkflowRepository{db: db}
}

func (r *WorkflowRepository) Save(ctx context.Context, workflow *model.Workflow) error {
	row := toRow(workflow)
	row.AuditColumns = postgres.AuditColumns{CreatedAt: workflow.CreatedAt, UpdatedAt: time.Now().UTC()}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *WorkflowRepository) Update(ctx context.Context, workflow *model.Workflow) error {
	return r.db.WithContext(ctx).Model(&workflowModel{}).Where("id = ?", workflow.ID.String()).Updates(map[string]any{
		"name":        workflow.Name.String(),
		"description": workflow.Description,
		"status":      string(workflow.Status),
		"definition":  workflow.Definition,
		"updated_at":  time.Now().UTC(),
	}).Error
}

func (r *WorkflowRepository) FindByID(ctx context.Context, id kernel.WorkflowID) (*model.Workflow, error) {
	var row workflowModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrWorkflowNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToModel(row)
}

func (r *WorkflowRepository) FindByProjectAndName(ctx context.Context, projectID kernel.ProjectID, name model.WorkflowName) (*model.Workflow, error) {
	var row workflowModel
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND name = ? AND status != ?", projectID.String(), name.String(), string(model.WorkflowStatusArchived)).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrWorkflowNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToModel(row)
}

func (r *WorkflowRepository) ListByProject(ctx context.Context, projectID kernel.ProjectID) ([]*model.Workflow, error) {
	var rows []workflowModel
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND status != ?", projectID.String(), string(model.WorkflowStatusArchived)).
		Order("created_at asc").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Workflow, 0, len(rows))
	for _, row := range rows {
		wf, err := rowToModel(row)
		if err != nil {
			return nil, err
		}
		out = append(out, wf)
	}
	return out, nil
}

func toRow(wf *model.Workflow) workflowModel {
	return workflowModel{
		ID: wf.ID.String(), ProjectID: wf.ProjectID.String(),
		Name: wf.Name.String(), Description: wf.Description,
		Status: string(wf.Status), Definition: wf.Definition,
	}
}

func rowToModel(row workflowModel) (*model.Workflow, error) {
	name, err := model.NewWorkflowName(row.Name)
	if err != nil {
		return nil, err
	}
	id, _ := kernel.ParseWorkflowID(row.ID)
	projectID, _ := kernel.ParseProjectID(row.ProjectID)
	return &model.Workflow{
		ID: id, ProjectID: projectID, Name: name,
		Description: row.Description, Status: model.WorkflowStatus(row.Status),
		Definition: row.Definition, CreatedAt: row.CreatedAt,
	}, nil
}

var _ repository.WorkflowRepository = (*WorkflowRepository)(nil)

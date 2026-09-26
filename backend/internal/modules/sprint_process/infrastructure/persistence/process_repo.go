package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type sprintProcessModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	ProductID   string
	Name        string
	TemplateKey string
	Description string `gorm:"type:text"`
	Version     int
	IsDefault   bool
	Stages      string `gorm:"type:jsonb"`
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (sprintProcessModel) TableName() string { return "sprint_process.sprint_processes" }

type workspaceSprintTemplateModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	WorkspaceID string
	Name        string
	TemplateKey string
	Description string `gorm:"type:text"`
	Stages      string `gorm:"type:jsonb"`
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (workspaceSprintTemplateModel) TableName() string { return "sprint_process.workspace_sprint_process_templates" }

type ProcessRepository struct{ db *gorm.DB }

func NewProcessRepository(db *gorm.DB) repository.ProcessRepository {
	return &ProcessRepository{db: db}
}

type WorkspaceTemplateRepository struct{ db *gorm.DB }

func NewWorkspaceTemplateRepository(db *gorm.DB) repository.WorkspaceTemplateRepository {
	return &WorkspaceTemplateRepository{db: db}
}

func (r *ProcessRepository) FindDefaultByProduct(ctx context.Context, productID kernel.ProductID) (*model.SprintProcess, error) {
	var row sprintProcessModel
	err := r.db.WithContext(ctx).
		Where("product_id = ? AND is_default = ?", productID.String(), true).
		Order("version DESC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSprintProcessNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToSprintProcess(row)
}

func (r *ProcessRepository) FindByID(ctx context.Context, productID kernel.ProductID, processID kernel.SprintProcessID) (*model.SprintProcess, error) {
	var row sprintProcessModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND product_id = ?", processID.String(), productID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSprintProcessNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToSprintProcess(row)
}

func (r *ProcessRepository) ListByProduct(ctx context.Context, productID kernel.ProductID) ([]*model.SprintProcess, error) {
	var rows []sprintProcessModel
	if err := r.db.WithContext(ctx).
		Where("product_id = ?", productID.String()).
		Order("is_default DESC, updated_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.SprintProcess, 0, len(rows))
	for _, row := range rows {
		p, err := rowToSprintProcess(row)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *ProcessRepository) Save(ctx context.Context, process *model.SprintProcess) error {
	row, err := toSprintProcessModel(process)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ProcessRepository) Update(ctx context.Context, process *model.SprintProcess) error {
	stages, err := json.Marshal(process.Stages)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&sprintProcessModel{}).Where("id = ?", process.ID.String()).Updates(map[string]any{
		"name": process.Name, "template_key": process.TemplateKey, "description": process.Description,
		"version": process.Version, "is_default": process.IsDefault, "stages": string(stages),
		"updated_at": time.Now().UTC(),
	}).Error
}

func (r *ProcessRepository) Delete(ctx context.Context, productID kernel.ProductID, processID kernel.SprintProcessID) error {
	var row sprintProcessModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND product_id = ?", processID.String(), productID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrSprintProcessNotFound
	}
	if err != nil {
		return err
	}
	if row.IsDefault {
		return domain.ErrSprintProcessDefaultProtected
	}
	return r.db.WithContext(ctx).Delete(&sprintProcessModel{}, "id = ?", processID.String()).Error
}

func (r *WorkspaceTemplateRepository) List(ctx context.Context, workspaceID kernel.WorkspaceID) ([]*model.WorkspaceSprintProcessTemplate, error) {
	var rows []workspaceSprintTemplateModel
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID.String()).
		Order("updated_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.WorkspaceSprintProcessTemplate, 0, len(rows))
	for _, row := range rows {
		tpl, err := rowToWorkspaceSprintTemplate(row)
		if err != nil {
			return nil, err
		}
		out = append(out, tpl)
	}
	return out, nil
}

func (r *WorkspaceTemplateRepository) Save(ctx context.Context, tpl *model.WorkspaceSprintProcessTemplate) error {
	row, err := toWorkspaceSprintTemplateModel(tpl)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *WorkspaceTemplateRepository) Update(ctx context.Context, tpl *model.WorkspaceSprintProcessTemplate) error {
	stages, err := json.Marshal(tpl.Stages)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&workspaceSprintTemplateModel{}).Where("id = ?", tpl.ID.String()).Updates(map[string]any{
		"name": tpl.Name, "template_key": tpl.TemplateKey, "description": tpl.Description,
		"stages": string(stages), "updated_at": time.Now().UTC(),
	}).Error
}

func (r *WorkspaceTemplateRepository) FindByID(ctx context.Context, workspaceID kernel.WorkspaceID, id kernel.WorkspaceSprintProcessTemplateID) (*model.WorkspaceSprintProcessTemplate, error) {
	var row workspaceSprintTemplateModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ?", id.String(), workspaceID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrWorkspaceSprintProcessTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToWorkspaceSprintTemplate(row)
}

func (r *WorkspaceTemplateRepository) Delete(ctx context.Context, workspaceID kernel.WorkspaceID, id kernel.WorkspaceSprintProcessTemplateID) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ?", id.String(), workspaceID.String()).
		Delete(&workspaceSprintTemplateModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrWorkspaceSprintProcessTemplateNotFound
	}
	return nil
}

func toSprintProcessModel(p *model.SprintProcess) (sprintProcessModel, error) {
	stages, err := json.Marshal(p.Stages)
	if err != nil {
		return sprintProcessModel{}, err
	}
	return sprintProcessModel{
		ID: p.ID.String(), ProductID: p.ProductID.String(), Name: p.Name,
		TemplateKey: p.TemplateKey, Description: p.Description,
		Version: p.Version, IsDefault: p.IsDefault, Stages: string(stages),
		CreatedBy: p.CreatedBy.String(), CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}, nil
}

func rowToSprintProcess(row sprintProcessModel) (*model.SprintProcess, error) {
	id, _ := kernel.ParseSprintProcessID(row.ID)
	pid, _ := kernel.ParseProductID(row.ProductID)
	cby, _ := kernel.ParseMemberID(row.CreatedBy)
	var stages []model.SprintStage
	if row.Stages != "" {
		_ = json.Unmarshal([]byte(row.Stages), &stages)
	}
	return &model.SprintProcess{
		ID: id, ProductID: pid, Name: row.Name, TemplateKey: row.TemplateKey, Description: row.Description,
		Version: row.Version, IsDefault: row.IsDefault,
		Stages: stages, CreatedBy: cby, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func toWorkspaceSprintTemplateModel(t *model.WorkspaceSprintProcessTemplate) (workspaceSprintTemplateModel, error) {
	stages, err := json.Marshal(t.Stages)
	if err != nil {
		return workspaceSprintTemplateModel{}, err
	}
	return workspaceSprintTemplateModel{
		ID: t.ID.String(), WorkspaceID: t.WorkspaceID.String(), Name: t.Name,
		TemplateKey: t.TemplateKey, Description: t.Description, Stages: string(stages),
		CreatedBy: t.CreatedBy.String(), CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}, nil
}

func rowToWorkspaceSprintTemplate(row workspaceSprintTemplateModel) (*model.WorkspaceSprintProcessTemplate, error) {
	id, _ := kernel.ParseWorkspaceSprintProcessTemplateID(row.ID)
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	cby, _ := kernel.ParseMemberID(row.CreatedBy)
	var stages []model.SprintStage
	if row.Stages != "" {
		_ = json.Unmarshal([]byte(row.Stages), &stages)
	}
	return &model.WorkspaceSprintProcessTemplate{
		ID: id, WorkspaceID: wsID, Name: row.Name, TemplateKey: row.TemplateKey, Description: row.Description,
		Stages: stages, CreatedBy: cby, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

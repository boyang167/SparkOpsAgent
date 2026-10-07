package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func AutoMigrate(db *gorm.DB) error {
	if !postgres.ShouldRunAutoMigrate(db) {
		return nil
	}
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS product").Error; err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(&workflowTemplateModel{}); err != nil {
		return err
	}
	return workflowAutoMigrate(db)
}

type workflowTemplateModel struct {
	ID           string `gorm:"column:id;primaryKey"`
	Scope        string
	WorkspaceID  *string
	Name         string
	Key          string `gorm:"column:key"`
	Description  string `gorm:"type:text"`
	Category     string
	ProductTypes string `gorm:"type:jsonb"`
	Status       string
	Version      string
	Definition   string `gorm:"type:jsonb"`
	SlotSchema   string `gorm:"type:jsonb"`
	KeyVals      string `gorm:"type:jsonb"`
	Runtime      string `gorm:"type:jsonb"`
	CreatedBy    string
	postgres.AuditColumns
	DeletedAt gorm.DeletedAt
}

func (workflowTemplateModel) TableName() string { return "product.workflow_templates" }

type WorkflowTemplateRepository struct{ db *gorm.DB }

func NewWorkflowTemplateRepository(db *gorm.DB) repository.WorkflowTemplateRepository {
	return &WorkflowTemplateRepository{db: db}
}

func (r *WorkflowTemplateRepository) Save(ctx context.Context, tpl *model.WorkflowTemplate) error {
	row, err := toTemplateModel(tpl)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *WorkflowTemplateRepository) Update(ctx context.Context, tpl *model.WorkflowTemplate) error {
	row, err := toTemplateModel(tpl)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&workflowTemplateModel{}).Where("id = ?", row.ID).Updates(map[string]any{
		"name": row.Name, "description": row.Description, "category": row.Category,
		"product_types": row.ProductTypes, "status": row.Status, "version": row.Version,
		"definition": row.Definition, "slot_schema": row.SlotSchema, "key_vals": row.KeyVals,
		"runtime": row.Runtime, "updated_at": time.Now().UTC(),
	}).Error
}

func (r *WorkflowTemplateRepository) FindByID(ctx context.Context, id kernel.WorkflowTemplateID) (*model.WorkflowTemplate, error) {
	var row workflowTemplateModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrWorkflowTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToTemplate(row)
}

func (r *WorkflowTemplateRepository) FindByKey(ctx context.Context, key string) (*model.WorkflowTemplate, error) {
	var row workflowTemplateModel
	err := r.db.WithContext(ctx).Where(`"key" = ?`, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrWorkflowTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToTemplate(row)
}

func (r *WorkflowTemplateRepository) ListForWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.WorkflowTemplate, error) {
	var rows []workflowTemplateModel
	err := r.db.WithContext(ctx).
		Where("scope = ? OR workspace_id = ?", string(model.TemplateScopeSystem), wsID.String()).
		Order("scope ASC, name ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.WorkflowTemplate, 0, len(rows))
	for _, row := range rows {
		tpl, err := rowToTemplate(row)
		if err != nil {
			return nil, err
		}
		out = append(out, tpl)
	}
	return out, nil
}

func toTemplateModel(tpl *model.WorkflowTemplate) (workflowTemplateModel, error) {
	productTypes, err := json.Marshal(tpl.ProductTypes)
	if err != nil {
		return workflowTemplateModel{}, err
	}
	keyVals, err := json.Marshal(tpl.KeyVals)
	if err != nil {
		return workflowTemplateModel{}, err
	}
	row := workflowTemplateModel{
		ID: tpl.ID.String(), Scope: string(tpl.Scope), Name: tpl.Name, Key: tpl.Key,
		Description: tpl.Description, Category: tpl.Category, ProductTypes: string(productTypes),
		Status: string(tpl.Status), Version: tpl.Version,
		Definition: string(tpl.Definition), SlotSchema: string(tpl.SlotSchema),
		KeyVals: string(keyVals), Runtime: string(tpl.Runtime),
		CreatedBy: tpl.CreatedBy.String(),
		AuditColumns: postgres.AuditColumns{CreatedAt: tpl.CreatedAt, UpdatedAt: tpl.UpdatedAt},
	}
	if tpl.WorkspaceID != nil {
		ws := tpl.WorkspaceID.String()
		row.WorkspaceID = &ws
	}
	return row, nil
}

func rowToTemplate(row workflowTemplateModel) (*model.WorkflowTemplate, error) {
	id, err := kernel.ParseWorkflowTemplateID(row.ID)
	if err != nil {
		return nil, err
	}
	createdBy, err := kernel.ParseMemberID(row.CreatedBy)
	if err != nil {
		return nil, err
	}
	var productTypes []string
	if row.ProductTypes != "" {
		_ = json.Unmarshal([]byte(row.ProductTypes), &productTypes)
	}
	var keyVals []model.KeyVal
	if row.KeyVals != "" {
		_ = json.Unmarshal([]byte(row.KeyVals), &keyVals)
	}
	tpl := &model.WorkflowTemplate{
		ID: id, Scope: model.TemplateScope(row.Scope), Name: row.Name, Key: row.Key,
		Description: row.Description, Category: row.Category, ProductTypes: productTypes,
		Status: model.TemplateStatus(row.Status), Version: row.Version,
		Definition: json.RawMessage(row.Definition), SlotSchema: json.RawMessage(row.SlotSchema),
		KeyVals: keyVals, Runtime: json.RawMessage(row.Runtime),
		CreatedBy: createdBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.WorkspaceID != nil && *row.WorkspaceID != "" {
		wsID, err := kernel.ParseWorkspaceID(*row.WorkspaceID)
		if err != nil {
			return nil, err
		}
		tpl.WorkspaceID = &wsID
	}
	return tpl, nil
}

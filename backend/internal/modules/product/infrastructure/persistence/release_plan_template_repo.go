package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type releasePlanTemplateModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	WorkspaceID string
	Name        string
	Description string `gorm:"type:text"`
	Snapshot    string `gorm:"type:jsonb"`
	CreatedBy   string
	postgres.AuditColumns
	DeletedAt gorm.DeletedAt
}

func (releasePlanTemplateModel) TableName() string { return "product.release_plan_templates" }

type ReleasePlanTemplateRepository struct{ db *gorm.DB }

func NewReleasePlanTemplateRepository(db *gorm.DB) repository.ReleasePlanTemplateRepository {
	return &ReleasePlanTemplateRepository{db: db}
}

func (r *ReleasePlanTemplateRepository) SaveTemplate(ctx context.Context, tpl *model.ReleasePlanTemplate) error {
	row, err := toReleasePlanTemplateModel(tpl)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ReleasePlanTemplateRepository) UpdateTemplate(ctx context.Context, tpl *model.ReleasePlanTemplate) error {
	row, err := toReleasePlanTemplateModel(tpl)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&releasePlanTemplateModel{}).Where("id = ?", row.ID).Updates(map[string]any{
		"name": row.Name, "description": row.Description, "snapshot": row.Snapshot, "updated_at": time.Now().UTC(),
	}).Error
}

func (r *ReleasePlanTemplateRepository) DeleteTemplate(ctx context.Context, id kernel.ReleasePlanTemplateID) error {
	return r.db.WithContext(ctx).Where("id = ?", id.String()).Delete(&releasePlanTemplateModel{}).Error
}

func (r *ReleasePlanTemplateRepository) FindTemplateByID(ctx context.Context, id kernel.ReleasePlanTemplateID) (*model.ReleasePlanTemplate, error) {
	var row releasePlanTemplateModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrReleasePlanTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToReleasePlanTemplate(row)
}

func (r *ReleasePlanTemplateRepository) ListTemplatesByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.ReleasePlanTemplate, error) {
	var rows []releasePlanTemplateModel
	err := r.db.WithContext(ctx).Where("workspace_id = ?", wsID.String()).Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.ReleasePlanTemplate, 0, len(rows))
	for _, row := range rows {
		tpl, err := rowToReleasePlanTemplate(row)
		if err != nil {
			return nil, err
		}
		out = append(out, tpl)
	}
	return out, nil
}

func toReleasePlanTemplateModel(tpl *model.ReleasePlanTemplate) (releasePlanTemplateModel, error) {
	raw, err := json.Marshal(tpl.Snapshot)
	if err != nil {
		return releasePlanTemplateModel{}, err
	}
	return releasePlanTemplateModel{
		ID: tpl.ID.String(), WorkspaceID: tpl.WorkspaceID.String(),
		Name: tpl.Name, Description: tpl.Description, Snapshot: string(raw),
		CreatedBy: tpl.CreatedBy.String(),
		AuditColumns: postgres.AuditColumns{CreatedAt: tpl.CreatedAt, UpdatedAt: tpl.UpdatedAt},
	}, nil
}

func rowToReleasePlanTemplate(row releasePlanTemplateModel) (*model.ReleasePlanTemplate, error) {
	id, err := kernel.ParseReleasePlanTemplateID(row.ID)
	if err != nil {
		return nil, err
	}
	wsID, err := kernel.ParseWorkspaceID(row.WorkspaceID)
	if err != nil {
		return nil, err
	}
	createdBy, err := kernel.ParseMemberID(row.CreatedBy)
	if err != nil {
		return nil, err
	}
	var snap model.ReleasePlanTemplateSnapshot
	_ = json.Unmarshal([]byte(row.Snapshot), &snap)
	return &model.ReleasePlanTemplate{
		ID: id, WorkspaceID: wsID, Name: row.Name, Description: row.Description,
		Snapshot: snap, CreatedBy: createdBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

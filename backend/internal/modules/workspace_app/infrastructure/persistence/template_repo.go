package persistence

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type templateModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	WorkspaceID string
	Name        string
	Kind        string
	Language    string
	Detail      string `gorm:"type:text"`
	Rules       string `gorm:"type:text"`
	Status      string
	IsPublic    bool `gorm:"column:is_public"`
	CreatedBy   string
	postgres.AuditColumns
}

func (templateModel) TableName() string { return "workspace_app.templates" }

type TemplateRepository struct{ db *gorm.DB }

func NewTemplateRepository(db *gorm.DB) repository.TemplateRepository {
	return &TemplateRepository{db: db}
}

func (r *TemplateRepository) Save(ctx context.Context, t *model.Template) error {
	row := templateToRow(t)
	row.AuditColumns = postgres.AuditColumns{CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *TemplateRepository) Update(ctx context.Context, t *model.Template) error {
	return r.db.WithContext(ctx).Model(&templateModel{}).
		Where("id = ? AND workspace_id = ?", t.ID.String(), t.WorkspaceID.String()).
		Updates(map[string]any{
			"name": t.Name.String(), "kind": t.Kind, "language": t.Language,
			"detail": t.Detail, "rules": t.Rules, "status": string(t.Status),
			"is_public": t.IsPublic, "updated_at": t.UpdatedAt,
		}).Error
}

func (r *TemplateRepository) Delete(ctx context.Context, id kernel.TemplateID) error {
	return r.db.WithContext(ctx).Where("id = ?", id.String()).Delete(&templateModel{}).Error
}

func (r *TemplateRepository) FindByID(ctx context.Context, wsID kernel.WorkspaceID, id kernel.TemplateID) (*model.Template, error) {
	var row templateModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ?", id.String(), wsID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToTemplate(row)
}

func (r *TemplateRepository) ListByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Template, error) {
	var rows []templateModel
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", wsID.String()).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Template, 0, len(rows))
	for _, row := range rows {
		t, err := rowToTemplate(row)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func templateToRow(t *model.Template) templateModel {
	return templateModel{
		ID: t.ID.String(), WorkspaceID: t.WorkspaceID.String(),
		Name: t.Name.String(), Kind: t.Kind, Language: t.Language,
		Detail: t.Detail, Rules: t.Rules, Status: string(t.Status),
		IsPublic: t.IsPublic, CreatedBy: t.CreatedBy.String(),
	}
}

func rowToTemplate(row templateModel) (*model.Template, error) {
	name, err := model.NewTemplateName(row.Name)
	if err != nil {
		return nil, err
	}
	id, _ := kernel.ParseTemplateID(row.ID)
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	createdBy, _ := kernel.ParseMemberID(row.CreatedBy)
	return &model.Template{
		ID: id, WorkspaceID: wsID, Name: name, Kind: row.Kind, Language: row.Language,
		Detail: row.Detail, Rules: row.Rules, Status: model.TemplateStatus(row.Status),
		IsPublic: row.IsPublic, CreatedBy: createdBy,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

var _ repository.TemplateRepository = (*TemplateRepository)(nil)

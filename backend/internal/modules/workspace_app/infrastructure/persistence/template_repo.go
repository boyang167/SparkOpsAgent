package persistence

import (
	"context"

	"gorm.io/gorm"

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
	CreatedBy   string
	postgres.AuditColumns
}

func (templateModel) TableName() string { return "workspace_app.templates" }

type TemplateRepository struct{ db *gorm.DB }

func NewTemplateRepository(db *gorm.DB) repository.TemplateRepository {
	return &TemplateRepository{db: db}
}

func (r *TemplateRepository) Save(ctx context.Context, t *model.Template) error {
	row := templateModel{
		ID: t.ID.String(), WorkspaceID: t.WorkspaceID.String(),
		Name: t.Name.String(), Kind: t.Kind, Language: t.Language,
		Detail: t.Detail, Rules: t.Rules, CreatedBy: t.CreatedBy.String(),
		AuditColumns: postgres.AuditColumns{CreatedAt: t.CreatedAt, UpdatedAt: t.CreatedAt},
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *TemplateRepository) ListByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Template, error) {
	var rows []templateModel
	if err := r.db.WithContext(ctx).Where("workspace_id = ?", wsID.String()).Find(&rows).Error; err != nil {
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
		Detail: row.Detail, Rules: row.Rules, CreatedBy: createdBy, CreatedAt: row.CreatedAt,
	}, nil
}

var _ repository.TemplateRepository = (*TemplateRepository)(nil)

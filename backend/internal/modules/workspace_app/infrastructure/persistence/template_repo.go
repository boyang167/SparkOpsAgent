package persistence

import (
	"context"
	"errors"

	"github.com/google/uuid"
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
	return postgres.DB(ctx, r.db).Create(&row).Error
}

func (r *TemplateRepository) Update(ctx context.Context, t *model.Template) error {
	return postgres.DB(ctx, r.db).Model(&templateModel{}).
		Where("id = ? AND workspace_id = ?", t.ID.String(), t.WorkspaceID.String()).
		Updates(map[string]any{
			"name": t.Name.String(), "kind": t.Kind, "language": t.Language,
			"detail": t.Detail, "rules": t.Rules, "status": string(t.Status),
			"is_public": t.IsPublic, "updated_at": t.UpdatedAt,
		}).Error
}

func (r *TemplateRepository) Delete(ctx context.Context, id kernel.TemplateID) error {
	return postgres.DB(ctx, r.db).Where("id = ?", id.String()).Delete(&templateModel{}).Error
}

func (r *TemplateRepository) FindByID(ctx context.Context, wsID kernel.WorkspaceID, id kernel.TemplateID) (*model.Template, error) {
	var row templateModel
	err := postgres.DB(ctx, r.db).
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
	if err := postgres.DB(ctx, r.db).
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

func (r *TemplateRepository) ListAccessible(ctx context.Context, wsID kernel.WorkspaceID, orgID kernel.OrgID) ([]*model.TemplateView, error) {
	if orgID.UUID == uuid.Nil {
		items, err := r.ListByWorkspace(ctx, wsID)
		if err != nil {
			return nil, err
		}
		out := make([]*model.TemplateView, 0, len(items))
		for _, item := range items {
			out = append(out, &model.TemplateView{Template: item})
		}
		return out, nil
	}

	orgWorkspaceIDs, err := r.listWorkspaceIDsByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if len(orgWorkspaceIDs) == 0 {
		return r.ListAccessible(ctx, wsID, kernel.OrgID{})
	}

	var rows []templateModel
	err = postgres.DB(ctx, r.db).
		Where("workspace_id = ?", wsID.String()).
		Or("is_public = ? AND workspace_id IN ?", true, orgWorkspaceIDs).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	nameByWS, err := r.workspaceNames(ctx, orgWorkspaceIDs)
	if err != nil {
		return nil, err
	}

	out := make([]*model.TemplateView, 0, len(rows))
	for _, row := range rows {
		t, err := rowToTemplate(row)
		if err != nil {
			return nil, err
		}
		out = append(out, &model.TemplateView{
			Template:      t,
			WorkspaceName: nameByWS[t.WorkspaceID.String()],
		})
	}
	return out, nil
}

func (r *TemplateRepository) FindAccessible(ctx context.Context, wsID kernel.WorkspaceID, orgID kernel.OrgID, id kernel.TemplateID) (*model.TemplateView, error) {
	var row templateModel
	err := postgres.DB(ctx, r.db).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	t, err := rowToTemplate(row)
	if err != nil {
		return nil, err
	}
	if t.WorkspaceID == wsID {
		return &model.TemplateView{Template: t}, nil
	}
	if !t.IsPublic {
		return nil, domain.ErrTemplateNotFound
	}
	if orgID.UUID == uuid.Nil {
		return nil, domain.ErrTemplateNotFound
	}
	var count int64
	err = postgres.DB(ctx, r.db).Table("tenant.workspaces").
		Where("id = ? AND org_id = ? AND deleted_at IS NULL", t.WorkspaceID.String(), orgID.String()).
		Count(&count).Error
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, domain.ErrTemplateNotFound
	}
	names, err := r.workspaceNames(ctx, []string{t.WorkspaceID.String()})
	if err != nil {
		return nil, err
	}
	return &model.TemplateView{
		Template:      t,
		WorkspaceName: names[t.WorkspaceID.String()],
	}, nil
}

func (r *TemplateRepository) listWorkspaceIDsByOrg(ctx context.Context, orgID kernel.OrgID) ([]string, error) {
	var ids []string
	err := postgres.DB(ctx, r.db).Table("tenant.workspaces").
		Where("org_id = ? AND deleted_at IS NULL", orgID.String()).
		Pluck("id", &ids).Error
	return ids, err
}

func (r *TemplateRepository) workspaceNames(ctx context.Context, workspaceIDs []string) (map[string]string, error) {
	out := make(map[string]string, len(workspaceIDs))
	if len(workspaceIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID   string
		Name string
	}
	err := postgres.DB(ctx, r.db).Table("tenant.workspaces").
		Select("id, name").
		Where("id IN ?", workspaceIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.Name
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

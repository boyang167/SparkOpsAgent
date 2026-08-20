package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func AutoMigrate(db *gorm.DB) error {
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS workspace_app").Error; err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(&projectModel{}, &templateModel{}, &secretModel{}, &dockerfileModel{}, &projectMemberModel{}); err != nil {
		return err
	}
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec(`
			UPDATE workspace_app.projects SET review_status = 'none'
			WHERE review_status IS NULL OR review_status = '';
			CREATE INDEX IF NOT EXISTS idx_projects_review_status
				ON workspace_app.projects(workspace_id, review_status);
			DO $$
			BEGIN
				IF NOT EXISTS (
					SELECT 1 FROM pg_constraint WHERE conname = 'chk_projects_review_status'
				) THEN
					ALTER TABLE workspace_app.projects
						ADD CONSTRAINT chk_projects_review_status
						CHECK (review_status IN ('none', 'pending', 'approved', 'rejected'));
				END IF;
			END $$;
		`).Error; err != nil {
			return err
		}
	}
	return nil
}

type projectModel struct {
	ID             string `gorm:"column:id;primaryKey"`
	WorkspaceID    string
	Name           string
	DisplayName    string
	Description    string
	Namespace      string
	Language       string
	GitURL         string
	SecretID       *string
	GitRefType     string
	GitRefValue    string
	DockerfilePath string
	TemplateID     *string
	Resources      []byte `gorm:"type:jsonb"`
	Status         string
	ReviewStatus   string `gorm:"default:none;not null"`
	SubmittedBy    *string
	SubmittedAt    *time.Time
	ReviewedBy     *string
	ReviewedAt     *time.Time
	ReviewComment  string `gorm:"default:'';not null"`
	CreatedBy      string
	postgres.AuditColumns
}

func (projectModel) TableName() string { return "workspace_app.projects" }

type ProjectRepository struct{ db *gorm.DB }

func NewProjectRepository(db *gorm.DB) repository.ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Save(ctx context.Context, project *model.Project) error {
	resources, err := model.EncodeProjectResources(project.Resources)
	if err != nil {
		return err
	}
	row := projectModel{
		ID: project.ID.String(), WorkspaceID: project.WorkspaceID.String(),
		Name: project.Name.String(), DisplayName: project.DisplayName, Description: project.Description,
		Namespace: project.Namespace, Language: project.Language, GitURL: project.GitURL,
		GitRefType: project.GitRefType, GitRefValue: project.GitRefValue, DockerfilePath: project.DockerfilePath,
		Resources: resources, Status: string(project.Status), CreatedBy: project.CreatedBy.String(),
		ReviewStatus: string(project.ReviewStatus), SubmittedAt: project.SubmittedAt,
		ReviewedAt: project.ReviewedAt, ReviewComment: project.ReviewComment,
		AuditColumns: postgres.AuditColumns{CreatedAt: project.CreatedAt, UpdatedAt: project.CreatedAt},
	}
	if project.SubmittedBy != nil {
		value := project.SubmittedBy.String()
		row.SubmittedBy = &value
	}
	if project.ReviewedBy != nil {
		value := project.ReviewedBy.String()
		row.ReviewedBy = &value
	}
	if project.SecretID != "" {
		row.SecretID = &project.SecretID
	}
	if project.TemplateID != "" {
		row.TemplateID = &project.TemplateID
	}
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *ProjectRepository) Update(ctx context.Context, project *model.Project) error {
	resources, err := model.EncodeProjectResources(project.Resources)
	if err != nil {
		return err
	}
	updates := map[string]any{
		"name":            project.Name.String(),
		"display_name":    project.DisplayName,
		"description":     project.Description,
		"namespace":       project.Namespace,
		"language":        project.Language,
		"git_url":         project.GitURL,
		"git_ref_type":    project.GitRefType,
		"git_ref_value":   project.GitRefValue,
		"dockerfile_path": project.DockerfilePath,
		"resources":       resources,
		"status":          string(project.Status),
		"review_status":   string(project.ReviewStatus),
		"submitted_at":    project.SubmittedAt,
		"reviewed_at":     project.ReviewedAt,
		"review_comment":  project.ReviewComment,
		"updated_at":      time.Now().UTC(),
	}
	if project.SubmittedBy != nil {
		updates["submitted_by"] = project.SubmittedBy.String()
	} else {
		updates["submitted_by"] = nil
	}
	if project.ReviewedBy != nil {
		updates["reviewed_by"] = project.ReviewedBy.String()
	} else {
		updates["reviewed_by"] = nil
	}
	if project.SecretID != "" {
		updates["secret_id"] = project.SecretID
	} else {
		updates["secret_id"] = nil
	}
	if project.TemplateID != "" {
		updates["template_id"] = project.TemplateID
	} else {
		updates["template_id"] = nil
	}
	return r.db.WithContext(ctx).Model(&projectModel{}).Where("id = ?", project.ID.String()).Updates(updates).Error
}

func (r *ProjectRepository) FindByID(ctx context.Context, id kernel.ProjectID) (*model.Project, error) {
	var row projectModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProjectNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToProject(row)
}

func (r *ProjectRepository) FindByWorkspaceAndName(ctx context.Context, wsID kernel.WorkspaceID, name model.ProjectName) (*model.Project, error) {
	var row projectModel
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND name = ? AND status = ?", wsID.String(), name.String(), string(model.ProjectStatusActive)).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProjectNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToProject(row)
}

func (r *ProjectRepository) ListByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Project, error) {
	var rows []projectModel
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND status = ?", wsID.String(), string(model.ProjectStatusActive)).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Project, 0, len(rows))
	for _, row := range rows {
		p, err := rowToProject(row)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *ProjectRepository) CountByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&projectModel{}).
		Where("workspace_id = ? AND status = ?", wsID.String(), string(model.ProjectStatusActive)).
		Count(&count).Error
	return int(count), err
}

func rowToProject(row projectModel) (*model.Project, error) {
	name, err := model.ProjectNameFromDB(row.Name)
	if err != nil {
		return nil, err
	}
	id, _ := kernel.ParseProjectID(row.ID)
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	createdBy, _ := kernel.ParseMemberID(row.CreatedBy)
	resources, err := model.DecodeProjectResources(row.Resources)
	if err != nil {
		return nil, err
	}
	p := &model.Project{
		ID: id, WorkspaceID: wsID, Name: name, DisplayName: row.DisplayName, Description: row.Description,
		Namespace: row.Namespace, Language: row.Language, GitURL: row.GitURL,
		GitRefType: row.GitRefType, GitRefValue: row.GitRefValue, DockerfilePath: row.DockerfilePath,
		Resources: resources, Status: model.ProjectStatus(row.Status), CreatedBy: createdBy, CreatedAt: row.CreatedAt,
		ReviewStatus: model.ProjectReviewStatus(row.ReviewStatus), SubmittedAt: row.SubmittedAt,
		ReviewedAt: row.ReviewedAt, ReviewComment: row.ReviewComment,
	}
	if p.ReviewStatus == "" {
		p.ReviewStatus = model.ProjectReviewNone
	}
	if row.SubmittedBy != nil {
		submittedBy, err := kernel.ParseMemberID(*row.SubmittedBy)
		if err != nil {
			return nil, err
		}
		p.SubmittedBy = &submittedBy
	}
	if row.ReviewedBy != nil {
		reviewedBy, err := kernel.ParseMemberID(*row.ReviewedBy)
		if err != nil {
			return nil, err
		}
		p.ReviewedBy = &reviewedBy
	}
	if row.SecretID != nil {
		p.SecretID = *row.SecretID
	}
	if row.TemplateID != nil {
		p.TemplateID = *row.TemplateID
	}
	return p, nil
}

var _ repository.ProjectRepository = (*ProjectRepository)(nil)

package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type projectVersionModel struct {
	ID              string `gorm:"column:id;primaryKey"`
	WorkspaceID     string `gorm:"not null"`
	ProjectID       string `gorm:"not null"`
	ProductID       *string
	VersionTag      string `gorm:"size:64;not null"`
	Status          string `gorm:"size:16;not null;default:ready"`
	Source          string `gorm:"size:16;not null;default:workflow"`
	WorkflowRunID   *string
	WorkflowJobName string `gorm:"size:128;not null;default:''"`
	GitRefType      string `gorm:"size:16;not null;default:branch"`
	GitRefValue     string `gorm:"size:255;not null;default:''"`
	Manifests       []byte `gorm:"type:jsonb;not null;default:'[]'"`
	Images          []byte `gorm:"type:jsonb;not null;default:'[]'"`
	ErrorMessage    string `gorm:"not null;default:''"`
	CreatedBy       string `gorm:"not null"`
	CreatedAt       time.Time `gorm:"not null"`
}

func (projectVersionModel) TableName() string { return "workspace_app.project_versions" }

type ProjectVersionRepository struct {
	db *gorm.DB
}

func NewProjectVersionRepository(db *gorm.DB) *ProjectVersionRepository {
	return &ProjectVersionRepository{db: db}
}

func (r *ProjectVersionRepository) Create(ctx context.Context, version *model.ProjectVersion) error {
	row, err := toProjectVersionRow(version)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrProjectVersionTagExists
		}
		return err
	}
	return nil
}

func (r *ProjectVersionRepository) FindByVersionID(ctx context.Context, versionID kernel.ProjectVersionID) (*model.ProjectVersion, error) {
	var row projectVersionModel
	err := r.db.WithContext(ctx).Where("id = ?", versionID.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProjectVersionNotFound
	}
	if err != nil {
		return nil, err
	}
	return fromProjectVersionRow(row)
}

func (r *ProjectVersionRepository) FindByID(ctx context.Context, projectID kernel.ProjectID, versionID kernel.ProjectVersionID) (*model.ProjectVersion, error) {
	var row projectVersionModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND project_id = ?", versionID.String(), projectID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProjectVersionNotFound
	}
	if err != nil {
		return nil, err
	}
	return fromProjectVersionRow(row)
}

func (r *ProjectVersionRepository) ListByProject(ctx context.Context, projectID kernel.ProjectID, limit, offset int) ([]*model.ProjectVersion, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []projectVersionModel
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID.String()).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProjectVersion, 0, len(rows))
	for _, row := range rows {
		item, err := fromProjectVersionRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func toProjectVersionRow(version *model.ProjectVersion) (*projectVersionModel, error) {
	manifests, err := json.Marshal(version.Manifests)
	if err != nil {
		return nil, err
	}
	images, err := json.Marshal(version.Images)
	if err != nil {
		return nil, err
	}
	var productID *string
	if version.ProductID != nil {
		value := version.ProductID.String()
		productID = &value
	}
	var workflowRunID *string
	if version.WorkflowRunID != nil {
		value := version.WorkflowRunID.String()
		workflowRunID = &value
	}
	return &projectVersionModel{
		ID: version.ID.String(), WorkspaceID: version.WorkspaceID.String(),
		ProjectID: version.ProjectID.String(), ProductID: productID,
		VersionTag: version.VersionTag, Status: string(version.Status),
		Source: version.Source, WorkflowRunID: workflowRunID,
		WorkflowJobName: version.WorkflowJobName,
		GitRefType: version.GitRefType, GitRefValue: version.GitRefValue,
		Manifests: manifests, Images: images, ErrorMessage: version.ErrorMessage,
		CreatedBy: version.CreatedBy.String(), CreatedAt: version.CreatedAt,
	}, nil
}

func fromProjectVersionRow(row projectVersionModel) (*model.ProjectVersion, error) {
	id, err := kernel.ParseProjectVersionID(row.ID)
	if err != nil {
		return nil, err
	}
	wsID, err := kernel.ParseWorkspaceID(row.WorkspaceID)
	if err != nil {
		return nil, err
	}
	projectID, err := kernel.ParseProjectID(row.ProjectID)
	if err != nil {
		return nil, err
	}
	createdBy, err := kernel.ParseMemberID(row.CreatedBy)
	if err != nil {
		return nil, err
	}
	version := &model.ProjectVersion{
		ID: id, WorkspaceID: wsID, ProjectID: projectID,
		VersionTag: row.VersionTag, Status: model.ProjectVersionStatus(row.Status),
		Source: row.Source, WorkflowJobName: row.WorkflowJobName,
		GitRefType: row.GitRefType, GitRefValue: row.GitRefValue,
		ErrorMessage: row.ErrorMessage, CreatedBy: createdBy, CreatedAt: row.CreatedAt,
	}
	if row.ProductID != nil && *row.ProductID != "" {
		productID, err := kernel.ParseProductID(*row.ProductID)
		if err != nil {
			return nil, err
		}
		version.ProductID = &productID
	}
	if row.WorkflowRunID != nil && *row.WorkflowRunID != "" {
		runID, err := kernel.ParseProductWorkflowRunID(*row.WorkflowRunID)
		if err != nil {
			return nil, err
		}
		version.WorkflowRunID = &runID
	}
	if len(row.Manifests) > 0 && string(row.Manifests) != "null" {
		_ = json.Unmarshal(row.Manifests, &version.Manifests)
	}
	if len(row.Images) > 0 && string(row.Images) != "null" {
		_ = json.Unmarshal(row.Images, &version.Images)
	}
	return version, nil
}

var _ repository.ProjectVersionRepository = (*ProjectVersionRepository)(nil)

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}

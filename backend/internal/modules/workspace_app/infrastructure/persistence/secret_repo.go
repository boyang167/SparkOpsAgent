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

type secretModel struct {
	ID             string `gorm:"column:id;primaryKey"`
	WorkspaceID    string
	Name           string
	DisplayName    string
	Description    string `gorm:"type:text"`
	ServiceType    string
	ServiceURL     string
	Username       string
	CredentialBlob string `gorm:"column:credential_blob;type:text"`
	Status         string
	IsDefault      bool
	IsShared       bool
	CreatedBy      string
	postgres.AuditColumns
}

func (secretModel) TableName() string { return "workspace_app.secrets" }

type SecretRepository struct{ db *gorm.DB }

func NewSecretRepository(db *gorm.DB) repository.SecretRepository {
	return &SecretRepository{db: db}
}

func (r *SecretRepository) Save(ctx context.Context, s *model.Secret) error {
	row := secretToRow(s)
	row.AuditColumns = postgres.AuditColumns{CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *SecretRepository) Update(ctx context.Context, s *model.Secret) error {
	return r.db.WithContext(ctx).Model(&secretModel{}).
		Where("id = ? AND workspace_id = ?", s.ID.String(), s.WorkspaceID.String()).
		Updates(map[string]any{
			"display_name": s.DisplayName, "description": s.Description,
			"service_type": s.ServiceType, "service_url": s.ServiceURL, "username": s.Username,
			"credential_blob": s.CredentialBlob, "status": string(s.Status),
			"is_default": s.IsDefault, "is_shared": s.IsShared, "updated_at": s.UpdatedAt,
		}).Error
}

func (r *SecretRepository) Delete(ctx context.Context, id kernel.SecretID) error {
	return r.db.WithContext(ctx).Where("id = ?", id.String()).Delete(&secretModel{}).Error
}

func (r *SecretRepository) FindByID(ctx context.Context, wsID kernel.WorkspaceID, id kernel.SecretID) (*model.Secret, error) {
	var row secretModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ?", id.String(), wsID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrSecretNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToSecret(row)
}

func (r *SecretRepository) ListByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Secret, error) {
	var rows []secretModel
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", wsID.String()).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Secret, 0, len(rows))
	for _, row := range rows {
		s, err := rowToSecret(row)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func secretToRow(s *model.Secret) secretModel {
	return secretModel{
		ID: s.ID.String(), WorkspaceID: s.WorkspaceID.String(), Name: s.Name.String(),
		DisplayName: s.DisplayName, Description: s.Description, ServiceType: s.ServiceType,
		ServiceURL: s.ServiceURL, Username: s.Username, CredentialBlob: s.CredentialBlob,
		Status: string(s.Status), IsDefault: s.IsDefault, IsShared: s.IsShared,
		CreatedBy: s.CreatedBy.String(),
	}
}

func rowToSecret(row secretModel) (*model.Secret, error) {
	name, err := model.NewSecretName(row.Name)
	if err != nil {
		return nil, err
	}
	id, _ := kernel.ParseSecretID(row.ID)
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	createdBy, _ := kernel.ParseMemberID(row.CreatedBy)
	return &model.Secret{
		ID: id, WorkspaceID: wsID, Name: name, DisplayName: row.DisplayName,
		Description: row.Description, ServiceType: row.ServiceType, ServiceURL: row.ServiceURL,
		Username: row.Username, CredentialBlob: row.CredentialBlob, Status: model.SecretStatus(row.Status),
		IsDefault: row.IsDefault, IsShared: row.IsShared, CreatedBy: createdBy,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

var _ repository.SecretRepository = (*SecretRepository)(nil)

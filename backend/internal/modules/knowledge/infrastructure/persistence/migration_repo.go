package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func (r *KnowledgeRepository) FindDocumentMigrationByID(ctx context.Context, documentID kernel.DocumentID) (*model.DocumentFileMigration, error) {
	var row documentFileMigrationModel
	err := r.db.WithContext(ctx).Where("document_id = ?", documentID.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return migrationRowToModel(row)
}

func (r *KnowledgeRepository) SaveDocumentMigration(ctx context.Context, row model.DocumentFileMigration) error {
	data := migrationModelFromDomain(row)
	data.UpdatedAt = time.Now().UTC()
	if data.CreatedAt.IsZero() {
		data.CreatedAt = data.UpdatedAt
	}
	return r.db.WithContext(ctx).Save(&data).Error
}

func (r *KnowledgeRepository) ListDocumentsForMigration(ctx context.Context, afterID *kernel.DocumentID, limit int) ([]*model.Document, error) {
	if limit <= 0 {
		limit = 100
	}
	query := r.db.WithContext(ctx).Model(&documentModel{}).Order("id asc")
	if afterID != nil {
		query = query.Where("id > ?", afterID.String())
	}
	var rows []documentModel
	if err := query.Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Document, 0, len(rows))
	for _, row := range rows {
		doc, err := docRowToModel(row)
		if err != nil {
			return nil, err
		}
		out = append(out, doc)
	}
	return out, nil
}

func (r *KnowledgeRepository) CountLegacyDocuments(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&documentModel{}).Count(&count).Error
	return count, err
}

func (r *KnowledgeRepository) CountDocumentMigrationsByStatus(ctx context.Context, status string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&documentFileMigrationModel{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

func (r *KnowledgeRepository) SetDocumentMigrationFields(ctx context.Context, documentID kernel.DocumentID, fileID *uuid.UUID, migrationStatus string) error {
	updates := map[string]any{"updated_at": time.Now().UTC()}
	if fileID != nil {
		v := fileID.String()
		updates["file_id"] = v
	}
	if migrationStatus != "" {
		updates["migration_status"] = migrationStatus
	}
	return r.db.WithContext(ctx).Model(&documentModel{}).Where("id = ?", documentID.String()).Updates(updates).Error
}

func (r *KnowledgeRepository) FindWorkspaceIDForDocument(ctx context.Context, documentID kernel.DocumentID) (kernel.WorkspaceID, error) {
	type row struct {
		WorkspaceID string
	}
	var result row
	err := r.db.WithContext(ctx).
		Table(documentModel{}.TableName()+" d").
		Select("kb.workspace_id").
		Joins("INNER JOIN "+knowledgeBaseModel{}.TableName()+" kb ON kb.id = d.knowledge_base_id").
		Where("d.id = ?", documentID.String()).
		Scan(&result).Error
	if err != nil {
		return kernel.WorkspaceID{}, err
	}
	if result.WorkspaceID == "" {
		return kernel.WorkspaceID{}, domain.ErrDocumentNotFound
	}
	return kernel.ParseWorkspaceID(result.WorkspaceID)
}

func migrationModelFromDomain(row model.DocumentFileMigration) documentFileMigrationModel {
	out := documentFileMigrationModel{
		DocumentID: row.DocumentID.String(), SourceSHA256: row.SourceSHA256,
		TargetSHA256: row.TargetSHA256, Status: row.Status,
		ErrorCode: row.ErrorCode, VerifiedAt: row.VerifiedAt,
	}
	if row.FileID != nil {
		v := row.FileID.String()
		out.FileID = &v
	}
	return out
}

func migrationRowToModel(row documentFileMigrationModel) (*model.DocumentFileMigration, error) {
	docID, err := uuid.Parse(row.DocumentID)
	if err != nil {
		return nil, err
	}
	out := &model.DocumentFileMigration{
		DocumentID: docID, SourceSHA256: row.SourceSHA256,
		TargetSHA256: row.TargetSHA256, Status: row.Status,
		ErrorCode: row.ErrorCode, VerifiedAt: row.VerifiedAt,
	}
	if row.FileID != nil && *row.FileID != "" {
		parsed, err := uuid.Parse(*row.FileID)
		if err != nil {
			return nil, err
		}
		out.FileID = &parsed
	}
	return out, nil
}

package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func AutoMigrate(db *gorm.DB) error {
	if !postgres.ShouldRunAutoMigrate(db) {
		return nil
	}
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS knowledge").Error; err != nil {
			return err
		}
	}
	return db.AutoMigrate(&knowledgeBaseModel{}, &documentModel{}, &documentFileMigrationModel{})
}

type knowledgeBaseModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	WorkspaceID string
	Name        string
	Description string
	Status      string
	postgres.AuditColumns
}

func (knowledgeBaseModel) TableName() string { return "knowledge.knowledge_bases" }

type documentModel struct {
	ID              string `gorm:"column:id;primaryKey"`
	KnowledgeBaseID string
	Title           string
	Content         string `gorm:"type:text"`
	FileID          *string
	MigrationStatus string
	Status          string
	postgres.AuditColumns
}

func (documentModel) TableName() string { return "knowledge.documents" }

type KnowledgeRepository struct{ db *gorm.DB }

func NewKnowledgeRepository(db *gorm.DB) repository.KnowledgeBaseRepository {
	return &KnowledgeRepository{db: db}
}

func (r *KnowledgeRepository) SaveKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error {
	row := kbToRow(kb)
	row.AuditColumns = postgres.AuditColumns{CreatedAt: kb.CreatedAt, UpdatedAt: time.Now().UTC()}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *KnowledgeRepository) UpdateKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error {
	return r.db.WithContext(ctx).Model(&knowledgeBaseModel{}).Where("id = ?", kb.ID.String()).Updates(map[string]any{
		"name":        kb.Name.String(),
		"description": kb.Description,
		"status":      string(kb.Status),
		"updated_at":  time.Now().UTC(),
	}).Error
}

func (r *KnowledgeRepository) FindKnowledgeBaseByID(ctx context.Context, id kernel.KnowledgeBaseID) (*model.KnowledgeBase, error) {
	var row knowledgeBaseModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrKnowledgeBaseNotFound
	}
	if err != nil {
		return nil, err
	}
	return kbRowToModel(row)
}

func (r *KnowledgeRepository) FindKnowledgeBaseByWorkspaceAndName(ctx context.Context, wsID kernel.WorkspaceID, name model.KnowledgeBaseName) (*model.KnowledgeBase, error) {
	var row knowledgeBaseModel
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND name = ? AND status != ?", wsID.String(), name.String(), string(model.KnowledgeBaseStatusArchived)).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrKnowledgeBaseNotFound
	}
	if err != nil {
		return nil, err
	}
	return kbRowToModel(row)
}

func (r *KnowledgeRepository) ListKnowledgeBasesByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.KnowledgeBase, error) {
	var rows []knowledgeBaseModel
	err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND status != ?", wsID.String(), string(model.KnowledgeBaseStatusArchived)).
		Order("created_at asc").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.KnowledgeBase, 0, len(rows))
	for _, row := range rows {
		kb, err := kbRowToModel(row)
		if err != nil {
			return nil, err
		}
		out = append(out, kb)
	}
	return out, nil
}

func (r *KnowledgeRepository) SaveDocument(ctx context.Context, doc *model.Document) error {
	row := docToRow(doc)
	row.AuditColumns = postgres.AuditColumns{CreatedAt: doc.CreatedAt, UpdatedAt: time.Now().UTC()}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *KnowledgeRepository) UpdateDocument(ctx context.Context, doc *model.Document) error {
	updates := map[string]any{
		"title":      doc.Title.String(),
		"content":    doc.Content,
		"status":     string(doc.Status),
		"updated_at": time.Now().UTC(),
	}
	if doc.FileID != nil {
		v := doc.FileID.String()
		updates["file_id"] = v
	}
	if doc.MigrationStatus != "" {
		updates["migration_status"] = doc.MigrationStatus
	}
	return r.db.WithContext(ctx).Model(&documentModel{}).Where("id = ?", doc.ID.String()).Updates(updates).Error
}

func (r *KnowledgeRepository) FindDocumentByID(ctx context.Context, id kernel.DocumentID) (*model.Document, error) {
	var row documentModel
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrDocumentNotFound
	}
	if err != nil {
		return nil, err
	}
	return docRowToModel(row)
}

func (r *KnowledgeRepository) FindDocumentByKBAndTitle(ctx context.Context, kbID kernel.KnowledgeBaseID, title model.DocumentTitle) (*model.Document, error) {
	var row documentModel
	err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND title = ? AND status != ?", kbID.String(), title.String(), string(model.DocumentStatusArchived)).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrDocumentNotFound
	}
	if err != nil {
		return nil, err
	}
	return docRowToModel(row)
}

func (r *KnowledgeRepository) ListDocumentsByKnowledgeBase(ctx context.Context, kbID kernel.KnowledgeBaseID) ([]*model.Document, error) {
	var rows []documentModel
	err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND status != ?", kbID.String(), string(model.DocumentStatusArchived)).
		Order("created_at asc").
		Find(&rows).Error
	if err != nil {
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

func (r *KnowledgeRepository) SearchDocumentsByKnowledgeBase(ctx context.Context, kbID kernel.KnowledgeBaseID, query string, limit int) ([]*model.Document, error) {
	if limit <= 0 {
		limit = 10
	}
	pattern := "%" + query + "%"
	var rows []documentModel
	err := r.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND status != ? AND (title ILIKE ? OR content ILIKE ?)",
			kbID.String(), string(model.DocumentStatusArchived), pattern, pattern).
		Order("updated_at desc").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
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

func kbToRow(kb *model.KnowledgeBase) knowledgeBaseModel {
	return knowledgeBaseModel{
		ID: kb.ID.String(), WorkspaceID: kb.WorkspaceID.String(),
		Name: kb.Name.String(), Description: kb.Description,
		Status: string(kb.Status),
	}
}

func kbRowToModel(row knowledgeBaseModel) (*model.KnowledgeBase, error) {
	name, err := model.NewKnowledgeBaseName(row.Name)
	if err != nil {
		return nil, err
	}
	id, _ := kernel.ParseKnowledgeBaseID(row.ID)
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	return &model.KnowledgeBase{
		ID: id, WorkspaceID: wsID, Name: name,
		Description: row.Description, Status: model.KnowledgeBaseStatus(row.Status),
		CreatedAt: row.CreatedAt,
	}, nil
}

func docToRow(doc *model.Document) documentModel {
	row := documentModel{
		ID: doc.ID.String(), KnowledgeBaseID: doc.KnowledgeBaseID.String(),
		Title: doc.Title.String(), Content: doc.Content,
		Status: string(doc.Status), MigrationStatus: doc.MigrationStatus,
	}
	if doc.FileID != nil {
		v := doc.FileID.String()
		row.FileID = &v
	}
	return row
}

func docRowToModel(row documentModel) (*model.Document, error) {
	title, err := model.NewDocumentTitle(row.Title)
	if err != nil {
		return nil, err
	}
	id, _ := kernel.ParseDocumentID(row.ID)
	kbID, _ := kernel.ParseKnowledgeBaseID(row.KnowledgeBaseID)
	doc := &model.Document{
		ID: id, KnowledgeBaseID: kbID, Title: title,
		Content: row.Content, Status: model.DocumentStatus(row.Status),
		MigrationStatus: row.MigrationStatus,
		CreatedAt:       row.CreatedAt,
	}
	if row.FileID != nil && *row.FileID != "" {
		parsed, err := uuid.Parse(*row.FileID)
		if err == nil {
			doc.FileID = &parsed
		}
	}
	return doc, nil
}

var _ repository.KnowledgeBaseRepository = (*KnowledgeRepository)(nil)

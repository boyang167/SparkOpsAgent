package acl

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/port"
)

type payloadRow struct {
	FileID  uuid.UUID `gorm:"column:file_id;primaryKey;type:uuid"`
	Payload []byte    `gorm:"not null"`
}

func (payloadRow) TableName() string { return "knowledge._migration_payloads" }

// FilesystemACL implements KnowledgeFileStore by staging bytes during migration
// and recording KNOWLEDGE_DOCUMENT file links for traceability.
type FilesystemACL struct {
	db       *gorm.DB
	payloads sync.Map
}

func NewFilesystemACL(db *gorm.DB) *FilesystemACL {
	acl := &FilesystemACL{db: db}
	if db != nil {
		_ = db.AutoMigrate(&payloadRow{})
		_ = db.Exec(`CREATE TABLE IF NOT EXISTS filesystem.file_links (
			id TEXT PRIMARY KEY,
			node_id TEXT NOT NULL,
			revision_id TEXT,
			link_type TEXT NOT NULL,
			link_id TEXT NOT NULL,
			created_at DATETIME
		)`).Error
	}
	return acl
}

var _ port.KnowledgeFileStore = (*FilesystemACL)(nil)

func (a *FilesystemACL) StoreDocument(ctx context.Context, in port.StoreDocumentInput) (port.StoredDocument, error) {
	mem := NewMemoryFileStore()
	stored, err := mem.StoreDocument(ctx, in)
	if err != nil {
		return port.StoredDocument{}, err
	}
	a.payloads.Store(stored.FileID, append([]byte(nil), in.Content...))
	if a.db != nil {
		now := time.Now().UTC()
		linkID := uuid.New()
		_ = a.db.WithContext(ctx).Exec(
			`INSERT INTO filesystem.file_links (id, node_id, link_type, link_id, created_at)
			 VALUES (?, ?, ?, ?, ?)`,
			linkID.String(), stored.FileID.String(), port.FileLinkTypeKnowledgeDocument,
			in.DocumentID.String(), now,
		).Error
		_ = a.db.WithContext(ctx).Save(&payloadRow{FileID: stored.FileID, Payload: in.Content}).Error
	}
	return stored, nil
}

func (a *FilesystemACL) ReadDocumentBytes(ctx context.Context, fileID uuid.UUID) ([]byte, error) {
	if raw, ok := a.payloads.Load(fileID); ok {
		return append([]byte(nil), raw.([]byte)...), nil
	}
	if a.db != nil {
		var row payloadRow
		if err := a.db.WithContext(ctx).Where("file_id = ?", fileID).First(&row).Error; err == nil {
			return append([]byte(nil), row.Payload...), nil
		}
	}
	return nil, context.DeadlineExceeded
}

func (a *FilesystemACL) CountFilesForDocument(ctx context.Context, documentID uuid.UUID) (int, error) {
	if a.db == nil {
		return 0, nil
	}
	var count int64
	err := a.db.WithContext(ctx).
		Table("filesystem.file_links").
		Where("link_type = ? AND link_id = ?", port.FileLinkTypeKnowledgeDocument, documentID.String()).
		Count(&count).Error
	return int(count), err
}

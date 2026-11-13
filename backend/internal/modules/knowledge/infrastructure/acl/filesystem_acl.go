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
	byDoc    sync.Map
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
	payload := append([]byte(nil), in.Content...)
	if fileID, ok := a.fileIDForDocument(ctx, in.DocumentID); ok {
		if a.db != nil {
			if err := a.db.WithContext(ctx).Save(&payloadRow{FileID: fileID, Payload: payload}).Error; err != nil {
				return port.StoredDocument{}, err
			}
		}
		a.remember(in.DocumentID, fileID, payload)
		return port.StoredDocument{FileID: fileID, RevisionID: uuid.New()}, nil
	}
	fileID := uuid.New()
	if a.db != nil {
		now := time.Now().UTC()
		linkID := uuid.New()
		if err := a.db.WithContext(ctx).Exec(
			`INSERT INTO filesystem.file_links (id, node_id, link_type, link_id, created_at)
			 VALUES (?, ?, ?, ?, ?)`,
			linkID.String(), fileID.String(), port.FileLinkTypeKnowledgeDocument,
			in.DocumentID.String(), now,
		).Error; err != nil {
			return port.StoredDocument{}, err
		}
		if err := a.db.WithContext(ctx).Save(&payloadRow{FileID: fileID, Payload: payload}).Error; err != nil {
			return port.StoredDocument{}, err
		}
	}
	a.remember(in.DocumentID, fileID, payload)
	return port.StoredDocument{FileID: fileID, RevisionID: uuid.New()}, nil
}

func (a *FilesystemACL) remember(documentID, fileID uuid.UUID, payload []byte) {
	a.byDoc.Store(documentID, fileID)
	a.payloads.Store(fileID, append([]byte(nil), payload...))
}

func (a *FilesystemACL) fileIDForDocument(ctx context.Context, documentID uuid.UUID) (uuid.UUID, bool) {
	if raw, ok := a.byDoc.Load(documentID); ok {
		id, _ := raw.(uuid.UUID)
		if id != uuid.Nil {
			return id, true
		}
	}
	if a.db == nil {
		return uuid.Nil, false
	}
	var nodeID string
	err := a.db.WithContext(ctx).Raw(
		`SELECT node_id FROM filesystem.file_links WHERE link_type = ? AND link_id = ? LIMIT 1`,
		port.FileLinkTypeKnowledgeDocument, documentID.String(),
	).Scan(&nodeID).Error
	if err != nil || nodeID == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(nodeID)
	if err != nil {
		return uuid.Nil, false
	}
	a.byDoc.Store(documentID, id)
	return id, true
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

package model

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type DocumentStatus string

const (
	DocumentStatusActive   DocumentStatus = "active"
	DocumentStatusArchived DocumentStatus = "archived"
)

type DocumentTitle struct {
	value string
}

func NewDocumentTitle(raw string) (DocumentTitle, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 255 {
		return DocumentTitle{}, domain.ErrInvalidDocumentTitle
	}
	return DocumentTitle{value: v}, nil
}

func (t DocumentTitle) String() string { return t.value }

type Document struct {
	ID              kernel.DocumentID
	KnowledgeBaseID kernel.KnowledgeBaseID
	Title           DocumentTitle
	Content         string
	FileID          *uuid.UUID
	MigrationStatus string
	Status          DocumentStatus
	CreatedAt       time.Time
}

func NewDocument(kbID kernel.KnowledgeBaseID, title DocumentTitle, content string) *Document {
	return &Document{
		ID:              kernel.NewDocumentID(),
		KnowledgeBaseID: kbID,
		Title:           title,
		Content:         content,
		Status:          DocumentStatusActive,
		CreatedAt:       time.Now().UTC(),
	}
}

func (d *Document) Update(title DocumentTitle, content string) error {
	if d.Status == DocumentStatusArchived {
		return domain.ErrDocumentArchived
	}
	d.Title = title
	d.Content = content
	return nil
}

func (d *Document) Archive() error {
	if d.Status == DocumentStatusArchived {
		return domain.ErrDocumentArchived
	}
	d.Status = DocumentStatusArchived
	return nil
}

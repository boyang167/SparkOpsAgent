package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type KnowledgeBaseRepository interface {
	SaveKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error
	UpdateKnowledgeBase(ctx context.Context, kb *model.KnowledgeBase) error
	FindKnowledgeBaseByID(ctx context.Context, id kernel.KnowledgeBaseID) (*model.KnowledgeBase, error)
	FindKnowledgeBaseByWorkspaceAndName(ctx context.Context, wsID kernel.WorkspaceID, name model.KnowledgeBaseName) (*model.KnowledgeBase, error)
	ListKnowledgeBasesByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.KnowledgeBase, error)

	SaveDocument(ctx context.Context, doc *model.Document) error
	UpdateDocument(ctx context.Context, doc *model.Document) error
	FindDocumentByID(ctx context.Context, id kernel.DocumentID) (*model.Document, error)
	FindDocumentByKBAndTitle(ctx context.Context, kbID kernel.KnowledgeBaseID, title model.DocumentTitle) (*model.Document, error)
	ListDocumentsByKnowledgeBase(ctx context.Context, kbID kernel.KnowledgeBaseID) ([]*model.Document, error)
	SearchDocumentsByKnowledgeBase(ctx context.Context, kbID kernel.KnowledgeBaseID, query string, limit int) ([]*model.Document, error)

	FindDocumentMigrationByID(ctx context.Context, documentID kernel.DocumentID) (*model.DocumentFileMigration, error)
	SaveDocumentMigration(ctx context.Context, row model.DocumentFileMigration) error
	ListDocumentsForMigration(ctx context.Context, afterID *kernel.DocumentID, limit int) ([]*model.Document, error)
	CountLegacyDocuments(ctx context.Context) (int64, error)
	CountDocumentMigrationsByStatus(ctx context.Context, status string) (int64, error)
	SetDocumentMigrationFields(ctx context.Context, documentID kernel.DocumentID, fileID *uuid.UUID, migrationStatus string) error
	FindWorkspaceIDForDocument(ctx context.Context, documentID kernel.DocumentID) (kernel.WorkspaceID, error)
}

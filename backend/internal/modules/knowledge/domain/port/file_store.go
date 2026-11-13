package port

import (
	"context"

	"github.com/google/uuid"
)

const FileLinkTypeKnowledgeDocument = "KNOWLEDGE_DOCUMENT"

type StoreDocumentInput struct {
	WorkspaceID uuid.UUID
	DocumentID  uuid.UUID
	Title       string
	Content     []byte
	MIMEType    string
}

type StoredDocument struct {
	FileID     uuid.UUID
	RevisionID uuid.UUID
}

// KnowledgeFileStore persists knowledge document bytes in the filesystem data plane.
type KnowledgeFileStore interface {
	StoreDocument(ctx context.Context, in StoreDocumentInput) (StoredDocument, error)
	ReadDocumentBytes(ctx context.Context, fileID uuid.UUID) ([]byte, error)
	CountFilesForDocument(ctx context.Context, documentID uuid.UUID) (int, error)
}

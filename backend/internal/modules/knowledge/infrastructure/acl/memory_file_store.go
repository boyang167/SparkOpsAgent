package acl

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/port"
)

// MemoryFileStore backs migration tests and local tooling without a full filesystem stack.
type MemoryFileStore struct {
	mu         sync.Mutex
	byDocument map[uuid.UUID]uuid.UUID
	byFile     map[uuid.UUID][]byte
}

func NewMemoryFileStore() *MemoryFileStore {
	return &MemoryFileStore{
		byDocument: make(map[uuid.UUID]uuid.UUID),
		byFile:     make(map[uuid.UUID][]byte),
	}
}

var _ port.KnowledgeFileStore = (*MemoryFileStore)(nil)

func (s *MemoryFileStore) StoreDocument(_ context.Context, in port.StoreDocumentInput) (port.StoredDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fileID, ok := s.byDocument[in.DocumentID]; ok {
		return port.StoredDocument{FileID: fileID, RevisionID: uuid.New()}, nil
	}
	fileID := uuid.New()
	copyContent := make([]byte, len(in.Content))
	copy(copyContent, in.Content)
	s.byDocument[in.DocumentID] = fileID
	s.byFile[fileID] = copyContent
	return port.StoredDocument{FileID: fileID, RevisionID: uuid.New()}, nil
}

func (s *MemoryFileStore) ReadDocumentBytes(_ context.Context, fileID uuid.UUID) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	payload, ok := s.byFile[fileID]
	if !ok {
		return nil, fmt.Errorf("file %s not found", fileID)
	}
	out := make([]byte, len(payload))
	copy(out, payload)
	return out, nil
}

func (s *MemoryFileStore) CountFilesForDocument(_ context.Context, documentID uuid.UUID) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byDocument[documentID]; ok {
		return 1, nil
	}
	return 0, nil
}

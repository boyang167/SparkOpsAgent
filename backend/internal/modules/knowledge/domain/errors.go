package domain

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

var (
	ErrInvalidKnowledgeBaseName = kernel.NewDomainError("KNOWLEDGE_INVALID_KB_NAME", "invalid knowledge base name")
	ErrKnowledgeBaseNotFound    = kernel.NewDomainError("KNOWLEDGE_KB_NOT_FOUND", "knowledge base not found")
	ErrKnowledgeBaseNameExists  = kernel.NewDomainError("KNOWLEDGE_KB_NAME_EXISTS", "knowledge base name already exists")
	ErrKnowledgeBaseArchived    = kernel.NewDomainError("KNOWLEDGE_KB_ARCHIVED", "knowledge base is archived")
	ErrInvalidDocumentTitle     = kernel.NewDomainError("KNOWLEDGE_INVALID_DOC_TITLE", "invalid document title")
	ErrDocumentNotFound         = kernel.NewDomainError("KNOWLEDGE_DOC_NOT_FOUND", "document not found")
	ErrDocumentTitleExists      = kernel.NewDomainError("KNOWLEDGE_DOC_TITLE_EXISTS", "document title already exists")
	ErrDocumentArchived         = kernel.NewDomainError("KNOWLEDGE_DOC_ARCHIVED", "document is archived")
	ErrWorkspaceAccessDenied    = kernel.NewDomainError("KNOWLEDGE_ACCESS_DENIED", "workspace access denied")
)

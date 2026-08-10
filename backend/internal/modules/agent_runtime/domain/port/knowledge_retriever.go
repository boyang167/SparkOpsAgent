package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type KnowledgeSnippet struct {
	DocumentID string
	Title      string
	Content    string
}

type KnowledgeRetriever interface {
	Retrieve(ctx context.Context, wsID kernel.WorkspaceID, kbIDs []string, query string, limit int) ([]KnowledgeSnippet, error)
}

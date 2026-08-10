package acl

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type KnowledgeACL struct{ db *gorm.DB }

func NewKnowledgeACL(db *gorm.DB) *KnowledgeACL { return &KnowledgeACL{db: db} }

type searchRow struct {
	ID      string
	Title   string
	Content string
}

func (k *KnowledgeACL) Retrieve(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	kbIDs []string,
	query string,
	limit int,
) ([]port.KnowledgeSnippet, error) {
	query = strings.TrimSpace(query)
	if query == "" || len(kbIDs) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}

	pattern := "%" + query + "%"
	var rows []searchRow
	err := k.db.WithContext(ctx).Table("knowledge.documents d").
		Select("d.id, d.title, d.content").
		Joins("JOIN knowledge.knowledge_bases kb ON kb.id = d.knowledge_base_id").
		Where("kb.workspace_id = ? AND kb.status != ? AND d.status != ?", wsID.String(), "archived", "archived").
		Where("d.knowledge_base_id IN ?", kbIDs).
		Where("(d.title ILIKE ? OR d.content ILIKE ?)", pattern, pattern).
		Order("d.updated_at desc").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]port.KnowledgeSnippet, 0, len(rows))
	for _, row := range rows {
		content := row.Content
		if len(content) > 1200 {
			content = content[:1200] + "..."
		}
		out = append(out, port.KnowledgeSnippet{
			DocumentID: row.ID,
			Title:      row.Title,
			Content:    content,
		})
	}
	return out, nil
}

func BuildRAGContext(snippets []port.KnowledgeSnippet) string {
	if len(snippets) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Use the following knowledge base excerpts when answering. If they are not relevant, ignore them.\n\n")
	for i, s := range snippets {
		b.WriteString(fmt.Sprintf("[Source %d: %s]\n%s\n\n", i+1, s.Title, s.Content))
	}
	return b.String()
}

var _ port.KnowledgeRetriever = (*KnowledgeACL)(nil)

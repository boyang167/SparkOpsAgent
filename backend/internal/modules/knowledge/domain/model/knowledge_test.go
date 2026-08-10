package model_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func mustKBName(t *testing.T, raw string) model.KnowledgeBaseName {
	t.Helper()
	name, err := model.NewKnowledgeBaseName(raw)
	require.NoError(t, err)
	return name
}

func mustDocTitle(t *testing.T, raw string) model.DocumentTitle {
	t.Helper()
	title, err := model.NewDocumentTitle(raw)
	require.NoError(t, err)
	return title
}

func TestKnowledgeBase_Archive(t *testing.T) {
	kb := model.NewKnowledgeBase(kernel.NewWorkspaceID(), mustKBName(t, "docs"), "desc")
	require.NoError(t, kb.Archive())
	require.Equal(t, model.KnowledgeBaseStatusArchived, kb.Status)
}

func TestKnowledgeBase_Update_AfterArchived(t *testing.T) {
	kb := model.NewKnowledgeBase(kernel.NewWorkspaceID(), mustKBName(t, "docs"), "")
	require.NoError(t, kb.Archive())
	err := kb.Update(mustKBName(t, "new"), "x")
	require.ErrorIs(t, err, domain.ErrKnowledgeBaseArchived)
}

func TestDocument_Update(t *testing.T) {
	doc := model.NewDocument(kernel.NewKnowledgeBaseID(), mustDocTitle(t, "intro"), "hello")
	require.NoError(t, doc.Update(mustDocTitle(t, "updated"), "world"))
	require.Equal(t, "updated", doc.Title.String())
	require.Equal(t, "world", doc.Content)
}

func TestDocument_Archive(t *testing.T) {
	doc := model.NewDocument(kernel.NewKnowledgeBaseID(), mustDocTitle(t, "intro"), "hello")
	require.NoError(t, doc.Archive())
	require.Equal(t, model.DocumentStatusArchived, doc.Status)
}

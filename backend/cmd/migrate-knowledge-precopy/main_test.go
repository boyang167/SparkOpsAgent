package main_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/persistence"
	knowledgemig "github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/migration"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestPrecopyIsIdempotentAndRecordsChecksum(t *testing.T) {
	db := openKnowledgeTestDB(t)
	repo := persistence.NewKnowledgeRepository(db)
	files := acl.NewMemoryFileStore()
	runner := knowledgemig.NewPrecopyRunner(repo, files)

	wsID := kernel.NewWorkspaceID()
	kb := model.NewKnowledgeBase(wsID, mustKBName(t, "docs"), "desc")
	require.NoError(t, repo.SaveKnowledgeBase(context.Background(), kb))

	content := "legacy knowledge body"
	doc := model.NewDocument(kb.ID, mustDocTitle(t, "intro"), content)
	require.NoError(t, repo.SaveDocument(context.Background(), doc))
	documentID := doc.ID
	sha256OfLegacyContent := knowledgemig.SHA256Hex([]byte(content))

	runPrecopy(t, runner, documentID)
	runPrecopy(t, runner, documentID)
	assertFileCount(t, files, documentID, 1)
	assertMappingHasSHA256(t, repo.(repository.KnowledgeBaseRepository), documentID, sha256OfLegacyContent)
}

func runPrecopy(t *testing.T, runner *knowledgemig.PrecopyRunner, documentID kernel.DocumentID) {
	t.Helper()
	require.NoError(t, runner.PrecopyDocument(context.Background(), documentID))
}

func assertFileCount(t *testing.T, files *acl.MemoryFileStore, documentID kernel.DocumentID, want int) {
	t.Helper()
	count, err := files.CountFilesForDocument(context.Background(), documentID.UUID)
	require.NoError(t, err)
	assert.Equal(t, want, count)
}

func assertMappingHasSHA256(t *testing.T, repo repository.KnowledgeBaseRepository, documentID kernel.DocumentID, want string) {
	t.Helper()
	mapping, err := repo.FindDocumentMigrationByID(context.Background(), documentID)
	require.NoError(t, err)
	require.NotNil(t, mapping)
	assert.Equal(t, model.FileMigrationStatusVerified, mapping.Status)
	assert.Equal(t, want, mapping.SourceSHA256)
	assert.Equal(t, want, mapping.TargetSHA256)
}

func openKnowledgeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS knowledge").Error)
	require.NoError(t, db.Exec(`CREATE TABLE knowledge.knowledge_bases (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		status TEXT NOT NULL,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE knowledge.documents (
		id TEXT PRIMARY KEY,
		knowledge_base_id TEXT NOT NULL,
		title TEXT NOT NULL,
		content TEXT,
		file_id TEXT,
		migration_status TEXT,
		status TEXT NOT NULL,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE knowledge.document_file_migrations (
		document_id TEXT PRIMARY KEY,
		file_id TEXT,
		source_sha256 TEXT NOT NULL,
		target_sha256 TEXT,
		status TEXT NOT NULL,
		error_code TEXT,
		verified_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error)
	return db
}

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

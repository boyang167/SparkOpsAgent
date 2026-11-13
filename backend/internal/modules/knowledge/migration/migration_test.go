package migration

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestPrecopyIsIdempotentWhenDocumentLinkCleared(t *testing.T) {
	db, repo, files, kbID := newHarness(t)
	content := "legacy knowledge body"
	doc := saveDoc(t, repo, kbID, "intro", content)
	runner := NewPrecopyRunner(repo, files)

	require.NoError(t, runner.PrecopyDocument(context.Background(), doc.ID))
	require.NoError(t, db.Exec(
		"UPDATE knowledge.documents SET file_id = NULL, migration_status = NULL WHERE id = ?",
		doc.ID.String(),
	).Error)
	require.NoError(t, runner.PrecopyDocument(context.Background(), doc.ID))

	count, err := files.CountFilesForDocument(context.Background(), doc.ID.UUID)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	loaded, err := repo.FindDocumentByID(context.Background(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, content, loaded.Content)
	require.NotNil(t, loaded.FileID)
	require.Equal(t, model.DocumentMigrationStatusPrecopy, loaded.MigrationStatus)

	mapping, err := repo.FindDocumentMigrationByID(context.Background(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, model.FileMigrationStatusVerified, mapping.Status)
	sum := SHA256Hex([]byte(content))
	require.Equal(t, sum, mapping.SourceSHA256)
	require.Equal(t, sum, mapping.TargetSHA256)
	require.Equal(t, loaded.FileID, mapping.FileID)
}

func TestPrecopyDryRunWritesNothing(t *testing.T) {
	_, repo, files, kbID := newHarness(t)
	doc := saveDoc(t, repo, kbID, "intro", "legacy knowledge body")
	runner := NewPrecopyRunner(repo, files)

	n, err := runner.RunBatch(context.Background(), PrecopyOptions{BatchSize: 100, DryRun: true})
	require.NoError(t, err)
	require.Equal(t, 1, n)

	count, err := files.CountFilesForDocument(context.Background(), doc.ID.UUID)
	require.NoError(t, err)
	require.Zero(t, count)
	mapping, err := repo.FindDocumentMigrationByID(context.Background(), doc.ID)
	require.NoError(t, err)
	require.Nil(t, mapping)
	loaded, err := repo.FindDocumentByID(context.Background(), doc.ID)
	require.NoError(t, err)
	require.Empty(t, loaded.MigrationStatus)
	require.Nil(t, loaded.FileID)
}

func TestPrecopyBatchResumesAfterID(t *testing.T) {
	_, repo, files, kbID := newHarness(t)
	saveDoc(t, repo, kbID, "a", "one")
	saveDoc(t, repo, kbID, "b", "two")
	docs, err := repo.ListDocumentsForMigration(context.Background(), nil, 10)
	require.NoError(t, err)
	require.Len(t, docs, 2)

	runner := NewPrecopyRunner(repo, files)
	n, err := runner.RunBatch(context.Background(), PrecopyOptions{BatchSize: 100, AfterID: &docs[0].ID})
	require.NoError(t, err)
	require.Equal(t, 1, n)

	skipped, err := repo.FindDocumentMigrationByID(context.Background(), docs[0].ID)
	require.NoError(t, err)
	require.Nil(t, skipped)
	copied, err := repo.FindDocumentMigrationByID(context.Background(), docs[1].ID)
	require.NoError(t, err)
	require.NotNil(t, copied)
	require.Equal(t, model.FileMigrationStatusVerified, copied.Status)
}

func TestPrecopyDoesNotVerifyMismatchedChecksum(t *testing.T) {
	_, repo, files, kbID := newHarness(t)
	doc := saveDoc(t, repo, kbID, "intro", "legacy knowledge body")
	runner := NewPrecopyRunner(repo, corruptReader{base: files})

	err := runner.PrecopyDocument(context.Background(), doc.ID)
	require.Error(t, err)

	mapping, err := repo.FindDocumentMigrationByID(context.Background(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, model.FileMigrationStatusFailed, mapping.Status)
	require.NotNil(t, mapping.ErrorCode)
	require.Equal(t, "CHECKSUM_MISMATCH", *mapping.ErrorCode)
	require.NotEqual(t, mapping.SourceSHA256, mapping.TargetSHA256)

	loaded, err := repo.FindDocumentByID(context.Background(), doc.ID)
	require.NoError(t, err)
	require.Empty(t, loaded.MigrationStatus)
	require.Nil(t, loaded.FileID)
}

func TestInventoryReportsFixedCounters(t *testing.T) {
	_, repo, _, kbID := newHarness(t)
	saveDoc(t, repo, kbID, "empty", "")
	saveDoc(t, repo, kbID, "alpha-1", "alpha")
	saveDoc(t, repo, kbID, "alpha-2", "alpha")
	saveDoc(t, repo, kbID, "bad", string([]byte{0xff, 0xfe}))
	saveDoc(t, repo, kernel.NewKnowledgeBaseID(), "orphan", "orphan-body")
	saveDoc(t, repo, kbID, "long", "0123456789abcdefghij")

	report, err := NewInventoryRunner(repo).Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(6), report.TotalDocuments)
	require.Equal(t, int64(1), report.EmptyContent)
	require.Equal(t, int64(43), report.TotalBytes)
	require.Equal(t, int64(20), report.MaxDocumentBytes)
	require.Equal(t, int64(1), report.EncodingAnomalies)
	require.Equal(t, int64(1), report.DuplicateSHA256)
	require.Equal(t, []string{SHA256Hex([]byte("alpha"))}, report.DuplicateSHA256Items)
	require.Equal(t, int64(1), report.MissingWorkspace)
	require.Equal(t, int64(5), report.EstimatedBlobCount)
}

func TestSwitchThenRollbackKeepsFileAndLegacyContent(t *testing.T) {
	_, repo, files, kbID := newHarness(t)
	content := "legacy knowledge body"
	doc := saveDoc(t, repo, kbID, "intro", content)
	require.NoError(t, NewPrecopyRunner(repo, files).PrecopyDocument(context.Background(), doc.ID))

	require.NoError(t, NewSwitchRunner(repo, files).FreezeSwitch(context.Background()))
	switched, err := repo.FindDocumentByID(context.Background(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, model.DocumentMigrationStatusSwitched, switched.MigrationStatus)
	require.NotNil(t, switched.FileID)
	require.Equal(t, content, switched.Content)

	require.NoError(t, NewSwitchRunner(repo, files).Rollback(context.Background()))
	rolled, err := repo.FindDocumentByID(context.Background(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, model.DocumentMigrationStatusRollback, rolled.MigrationStatus)
	require.Equal(t, switched.FileID, rolled.FileID)
	require.Equal(t, content, rolled.Content)

	count, err := files.CountFilesForDocument(context.Background(), doc.ID.UUID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	body, err := files.ReadDocumentBytes(context.Background(), *rolled.FileID)
	require.NoError(t, err)
	require.Equal(t, content, string(body))

	mapping, err := repo.FindDocumentMigrationByID(context.Background(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, model.FileMigrationStatusVerified, mapping.Status)
}

func TestSwitchBlockedUnlessFullyVerified(t *testing.T) {
	_, repo, files, kbID := newHarness(t)
	doc := saveDoc(t, repo, kbID, "intro", "legacy knowledge body")

	err := NewSwitchRunner(repo, files).FreezeSwitch(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, err, "switch blocked")

	loaded, err := repo.FindDocumentByID(context.Background(), doc.ID)
	require.NoError(t, err)
	require.Empty(t, loaded.MigrationStatus)
	require.Nil(t, loaded.FileID)
}

func TestSwitchBlockedWhenTargetUnreadable(t *testing.T) {
	_, repo, files, kbID := newHarness(t)
	doc := saveDoc(t, repo, kbID, "intro", "legacy knowledge body")
	require.NoError(t, NewPrecopyRunner(repo, files).PrecopyDocument(context.Background(), doc.ID))

	err := NewSwitchRunner(repo, unreadableReader{base: files}).FreezeSwitch(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, err, "unreadable")

	loaded, err := repo.FindDocumentByID(context.Background(), doc.ID)
	require.NoError(t, err)
	require.Equal(t, model.DocumentMigrationStatusPrecopy, loaded.MigrationStatus)
}

type corruptReader struct {
	base *acl.MemoryFileStore
}

func (c corruptReader) StoreDocument(ctx context.Context, in port.StoreDocumentInput) (port.StoredDocument, error) {
	return c.base.StoreDocument(ctx, in)
}

func (c corruptReader) ReadDocumentBytes(ctx context.Context, fileID uuid.UUID) ([]byte, error) {
	body, err := c.base.ReadDocumentBytes(ctx, fileID)
	if err != nil {
		return nil, err
	}
	return append(body, '!'), nil
}

func (c corruptReader) CountFilesForDocument(ctx context.Context, documentID uuid.UUID) (int, error) {
	return c.base.CountFilesForDocument(ctx, documentID)
}

type unreadableReader struct {
	base *acl.MemoryFileStore
}

func (u unreadableReader) StoreDocument(ctx context.Context, in port.StoreDocumentInput) (port.StoredDocument, error) {
	return u.base.StoreDocument(ctx, in)
}

func (u unreadableReader) ReadDocumentBytes(context.Context, uuid.UUID) ([]byte, error) {
	return nil, errors.New("unreadable")
}

func (u unreadableReader) CountFilesForDocument(ctx context.Context, documentID uuid.UUID) (int, error) {
	return u.base.CountFilesForDocument(ctx, documentID)
}

func newHarness(t *testing.T) (*gorm.DB, repository.KnowledgeBaseRepository, *acl.MemoryFileStore, kernel.KnowledgeBaseID) {
	t.Helper()
	db := openMigrationDB(t)
	repo := persistence.NewKnowledgeRepository(db)
	files := acl.NewMemoryFileStore()
	kb := model.NewKnowledgeBase(kernel.NewWorkspaceID(), mustKBName(t, "docs"), "desc")
	require.NoError(t, repo.SaveKnowledgeBase(context.Background(), kb))
	return db, repo, files, kb.ID
}

func saveDoc(t *testing.T, repo repository.KnowledgeBaseRepository, kbID kernel.KnowledgeBaseID, title, content string) *model.Document {
	t.Helper()
	doc := model.NewDocument(kbID, mustDocTitle(t, title), content)
	require.NoError(t, repo.SaveDocument(context.Background(), doc))
	return doc
}

func openMigrationDB(t *testing.T) *gorm.DB {
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

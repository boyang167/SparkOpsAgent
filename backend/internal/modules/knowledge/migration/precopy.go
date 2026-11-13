package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type PrecopyOptions struct {
	BatchSize int
	AfterID   *kernel.DocumentID
	DryRun    bool
}

type PrecopyRunner struct {
	repo  repository.KnowledgeBaseRepository
	files port.KnowledgeFileStore
}

func NewPrecopyRunner(repo repository.KnowledgeBaseRepository, files port.KnowledgeFileStore) *PrecopyRunner {
	return &PrecopyRunner{repo: repo, files: files}
}

func SHA256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func (r *PrecopyRunner) PrecopyDocument(ctx context.Context, documentID kernel.DocumentID) error {
	doc, err := r.repo.FindDocumentByID(ctx, documentID)
	if err != nil {
		return err
	}
	sourceSHA := SHA256Hex([]byte(doc.Content))
	existing, err := r.repo.FindDocumentMigrationByID(ctx, documentID)
	if err != nil {
		return err
	}
	if existing != nil && existing.Status == model.FileMigrationStatusVerified && existing.SourceSHA256 == sourceSHA {
		if optsDryRun(ctx) {
			return nil
		}
		if existing.FileID == nil {
			return fmt.Errorf("verified migration missing file_id")
		}
		return r.repo.SetDocumentMigrationFields(ctx, documentID, existing.FileID, model.DocumentMigrationStatusPrecopy)
	}
	if existing != nil && existing.FileID != nil {
		return r.verifyExisting(ctx, documentID, existing, sourceSHA)
	}
	if r.files == nil {
		return fmt.Errorf("knowledge file store is required")
	}
	wsID, err := r.repo.FindWorkspaceIDForDocument(ctx, documentID)
	if err != nil {
		return err
	}
	if optsDryRun(ctx) {
		return nil
	}
	stored, err := r.files.StoreDocument(ctx, port.StoreDocumentInput{
		WorkspaceID: wsID.UUID,
		DocumentID:  documentID.UUID,
		Title:       doc.Title.String(),
		Content:     []byte(doc.Content),
		MIMEType:    "text/plain; charset=utf-8",
	})
	if err != nil {
		return err
	}
	fileID := stored.FileID
	_ = stored.RevisionID
	mapping := model.DocumentFileMigration{
		DocumentID: documentID.UUID, FileID: &fileID,
		SourceSHA256: sourceSHA, Status: model.FileMigrationStatusPending,
	}
	if err := r.repo.SaveDocumentMigration(ctx, mapping); err != nil {
		return err
	}
	return r.verifyStored(ctx, documentID, fileID, sourceSHA)
}

func (r *PrecopyRunner) verifyExisting(ctx context.Context, documentID kernel.DocumentID, existing *model.DocumentFileMigration, sourceSHA string) error {
	if existing.FileID == nil {
		return fmt.Errorf("migration mapping missing file_id")
	}
	return r.verifyStored(ctx, documentID, *existing.FileID, sourceSHA)
}

func (r *PrecopyRunner) verifyStored(ctx context.Context, documentID kernel.DocumentID, fileID uuid.UUID, sourceSHA string) error {
	bytes, err := r.files.ReadDocumentBytes(ctx, fileID)
	if err != nil {
		failed := model.FileMigrationStatusFailed
		code := "READ_FAILED"
		_ = r.repo.SaveDocumentMigration(ctx, model.DocumentFileMigration{
			DocumentID: documentID.UUID, FileID: &fileID,
			SourceSHA256: sourceSHA, Status: failed, ErrorCode: &code,
		})
		return err
	}
	targetSHA := SHA256Hex(bytes)
	now := time.Now().UTC()
	status := model.FileMigrationStatusVerified
	mapping := model.DocumentFileMigration{
		DocumentID: documentID.UUID, FileID: &fileID,
		SourceSHA256: sourceSHA, TargetSHA256: targetSHA,
		Status: status, VerifiedAt: &now,
	}
	if targetSHA != sourceSHA {
		code := "CHECKSUM_MISMATCH"
		mapping.Status = model.FileMigrationStatusFailed
		mapping.ErrorCode = &code
		_ = r.repo.SaveDocumentMigration(ctx, mapping)
		return fmt.Errorf("checksum mismatch for document %s", documentID)
	}
	if err := r.repo.SaveDocumentMigration(ctx, mapping); err != nil {
		return err
	}
	return r.repo.SetDocumentMigrationFields(ctx, documentID, &fileID, model.DocumentMigrationStatusPrecopy)
}

func (r *PrecopyRunner) RunBatch(ctx context.Context, opts PrecopyOptions) (int, error) {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 100
	}
	docs, err := r.repo.ListDocumentsForMigration(ctx, opts.AfterID, opts.BatchSize)
	if err != nil {
		return 0, err
	}
	ctx = context.WithValue(ctx, dryRunContextKey{}, opts.DryRun)
	processed := 0
	for _, doc := range docs {
		if err := r.PrecopyDocument(ctx, doc.ID); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

type dryRunContextKey struct{}

func optsDryRun(ctx context.Context) bool {
	v, ok := ctx.Value(dryRunContextKey{}).(bool)
	return ok && v
}

func WithDryRun(ctx context.Context, dryRun bool) context.Context {
	return context.WithValue(ctx, dryRunContextKey{}, dryRun)
}

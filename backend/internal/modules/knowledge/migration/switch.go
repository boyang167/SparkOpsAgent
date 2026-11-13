package migration

import (
	"context"
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/repository"
)

type SwitchRunner struct {
	repo  repository.KnowledgeBaseRepository
	files port.KnowledgeFileStore
}

func NewSwitchRunner(repo repository.KnowledgeBaseRepository, files port.KnowledgeFileStore) *SwitchRunner {
	return &SwitchRunner{repo: repo, files: files}
}

func (r *SwitchRunner) FreezeSwitch(ctx context.Context) error {
	legacy, err := r.repo.CountLegacyDocuments(ctx)
	if err != nil {
		return err
	}
	verified, err := r.repo.CountDocumentMigrationsByStatus(ctx, model.FileMigrationStatusVerified)
	if err != nil {
		return err
	}
	failed, err := r.repo.CountDocumentMigrationsByStatus(ctx, model.FileMigrationStatusFailed)
	if err != nil {
		return err
	}
	if legacy != verified || failed != 0 {
		return fmt.Errorf("switch blocked: legacy=%d verified=%d failed=%d", legacy, verified, failed)
	}
	docs, err := r.repo.ListDocumentsForMigration(ctx, nil, 1_000_000)
	if err != nil {
		return err
	}
	for _, doc := range docs {
		mapping, err := r.repo.FindDocumentMigrationByID(ctx, doc.ID)
		if err != nil || mapping == nil || mapping.FileID == nil {
			return fmt.Errorf("document %s missing verified mapping", doc.ID)
		}
		if _, err := r.files.ReadDocumentBytes(ctx, *mapping.FileID); err != nil {
			return fmt.Errorf("target file unreadable for document %s: %w", doc.ID, err)
		}
		if err := r.repo.SetDocumentMigrationFields(ctx, doc.ID, mapping.FileID, model.DocumentMigrationStatusSwitched); err != nil {
			return err
		}
	}
	return nil
}

func (r *SwitchRunner) Rollback(ctx context.Context) error {
	docs, err := r.repo.ListDocumentsForMigration(ctx, nil, 1_000_000)
	if err != nil {
		return err
	}
	for _, doc := range docs {
		if err := r.repo.SetDocumentMigrationFields(ctx, doc.ID, doc.FileID, model.DocumentMigrationStatusRollback); err != nil {
			return err
		}
	}
	return nil
}

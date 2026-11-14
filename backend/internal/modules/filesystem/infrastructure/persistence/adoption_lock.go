package persistence

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func (r *Repository) LockNamespaceByOrganization(ctx context.Context, organizationID uuid.UUID) (*model.Namespace, error) {
	tx, err := postgres.RequireTx(ctx)
	if err != nil {
		return nil, domain.ErrPersistence
	}
	query := tx.Where("namespace_type = ? AND organization_id = ?", model.NamespaceTypeOrganization, organizationID)
	if tx.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	} else {
		result := tx.Exec(
			"UPDATE filesystem.namespaces SET updated_at = updated_at WHERE namespace_type = ? AND organization_id = ?",
			model.NamespaceTypeOrganization, organizationID,
		)
		if result.Error != nil {
			return nil, wrapPersistence(result.Error)
		}
	}
	var row namespaceRow
	err = query.First(&row).Error
	return namespaceResult(row, err)
}

func (r *Repository) LockNodeByID(ctx context.Context, nodeID uuid.UUID) (*model.Node, error) {
	return r.lockNode(ctx, uuid.Nil, nodeID)
}

func (r *Repository) LockNode(ctx context.Context, namespaceID, nodeID uuid.UUID) (*model.Node, error) {
	return r.lockNode(ctx, namespaceID, nodeID)
}

func (r *Repository) lockNode(ctx context.Context, namespaceID, nodeID uuid.UUID) (*model.Node, error) {
	tx, err := postgres.RequireTx(ctx)
	if err != nil {
		return nil, domain.ErrPersistence
	}
	query := tx.Where("id = ?", nodeID)
	if namespaceID != uuid.Nil {
		query = query.Where("namespace_id = ?", namespaceID)
	}
	if tx.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	} else {
		update := tx.Exec("UPDATE filesystem.nodes SET updated_at = updated_at WHERE id = ?", nodeID)
		if update.Error != nil {
			return nil, wrapPersistence(update.Error)
		}
		if update.RowsAffected == 0 {
			return nil, domain.ErrNodeNotFound
		}
	}
	var row nodeRow
	err = query.First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNodeNotFound
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	return nodeFromRow(row), nil
}

func (r *Repository) LockBlob(ctx context.Context, blobID uuid.UUID) (*model.Blob, error) {
	tx, err := postgres.RequireTx(ctx)
	if err != nil {
		return nil, domain.ErrPersistence
	}
	query := tx.Where("id = ?", blobID)
	if tx.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	} else {
		result := tx.Exec("UPDATE filesystem.blobs SET ref_count = ref_count WHERE id = ?", blobID)
		if result.Error != nil {
			return nil, wrapPersistence(result.Error)
		}
		if result.RowsAffected == 0 {
			return nil, domain.ErrBlobConflict
		}
	}
	var row blobRow
	err = query.First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrBlobConflict
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	return &model.Blob{
		ID: row.ID, DedupDomain: row.DedupDomain, SHA256: row.SHA256, Size: row.Size,
		EncryptionKeyRef: row.EncryptionKeyRef, RefCount: row.RefCount, State: row.State,
		PreferredProviderID: row.PreferredProviderID, TombstonedAt: row.TombstonedAt, LegalHold: row.LegalHold,
	}, nil
}

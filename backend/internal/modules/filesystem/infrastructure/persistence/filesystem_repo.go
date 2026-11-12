package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) repository.FilesystemRepository { return &Repository{db: db} }

func (r *Repository) CreateNamespace(ctx context.Context, namespace *model.Namespace) error {
	now := time.Now().UTC()
	status := namespace.Status
	if status == "" {
		status = "ACTIVE"
	}
	return r.db.WithContext(ctx).Create(&namespaceRow{
		ID: namespace.ID, OrganizationID: namespace.OrganizationID, WorkspaceID: namespace.WorkspaceID,
		RootNodeID: namespace.RootNodeID, Status: status, CreatedAt: now, UpdatedAt: now,
	}).Error
}

func (r *Repository) GetNamespaceByWorkspace(ctx context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
	var row namespaceRow
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNamespaceNotFound
	}
	if err != nil {
		return nil, err
	}
	return &model.Namespace{ID: row.ID, OrganizationID: row.OrganizationID, WorkspaceID: row.WorkspaceID, RootNodeID: row.RootNodeID, Status: row.Status}, nil
}

func (r *Repository) CreateNode(ctx context.Context, node *model.Node) error {
	db := r.db.WithContext(ctx)
	if node.ParentID != nil {
		var parent nodeRow
		err := db.Where("id = ? AND namespace_id = ?", *node.ParentID, node.NamespaceID).First(&parent).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNodeNotFound
		}
		if err != nil {
			return err
		}
	}
	query := db.Where("namespace_id = ? AND normalized_name = ?", node.NamespaceID, node.NormalizedName)
	if node.ParentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *node.ParentID)
	}
	var existing nodeRow
	if err := query.First(&existing).Error; err == nil {
		return domain.ErrNodeNameConflict
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	now := time.Now().UTC()
	err := db.Create(&nodeRow{
		ID: node.ID, NamespaceID: node.NamespaceID, ParentID: node.ParentID, Type: node.Type,
		Name: node.Name, NormalizedName: node.NormalizedName, CurrentRevisionID: node.CurrentRevisionID,
		State: node.State, OwnerID: node.OwnerID, CreatedAt: now, UpdatedAt: now,
	}).Error
	if isUniqueViolation(err) {
		return domain.ErrNodeNameConflict
	}
	return err
}

func (r *Repository) GetNode(ctx context.Context, namespaceID, nodeID uuid.UUID) (*model.Node, error) {
	var row nodeRow
	err := r.db.WithContext(ctx).Where("namespace_id = ? AND id = ?", namespaceID, nodeID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNodeNotFound
	}
	if err != nil {
		return nil, err
	}
	return nodeFromRow(row), nil
}

func (r *Repository) ListChildren(ctx context.Context, namespaceID uuid.UUID, parentID *uuid.UUID, limit int, cursor string) ([]model.Node, string, error) {
	query := r.db.WithContext(ctx).Where("namespace_id = ?", namespaceID)
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", *parentID)
	}
	if cursor != "" {
		query = query.Where("id > ?", cursor)
	}
	if limit <= 0 {
		limit = 100
	}
	var rows []nodeRow
	if err := query.Order("id ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", err
	}
	next := ""
	if len(rows) > limit {
		next = rows[limit-1].ID.String()
		rows = rows[:limit]
	}
	out := make([]model.Node, len(rows))
	for i := range rows {
		out[i] = *nodeFromRow(rows[i])
	}
	return out, next, nil
}

func (r *Repository) SaveRevision(ctx context.Context, revision *model.FileRevision, blob *model.Blob, replica *model.StorageReplica) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := tx.Create(&blobRow{ID: blob.ID, DedupDomain: blob.DedupDomain, SHA256: blob.SHA256, Size: blob.Size, EncryptionKeyRef: blob.EncryptionKeyRef, RefCount: blob.RefCount, State: blob.State, CreatedAt: now}).Error; err != nil {
			if !isUniqueViolation(err) {
				return err
			}
		}
		if err := tx.Create(&revisionRow{ID: revision.ID(), NodeID: revision.NodeID(), RevisionNo: revision.RevisionNo(), BlobID: revision.BlobID(), Size: revision.Size(), MIMEType: revision.MIMEType(), CreatedBy: revision.CreatedBy(), CreatedAt: now}).Error; err != nil {
			return err
		}
		return tx.Create(&replicaRow{ID: replica.ID, BlobID: replica.BlobID, ProviderID: replica.ProviderID, ObjectKey: replica.ObjectKey, Checksum: replica.Checksum, State: replica.State, CreatedAt: now}).Error
	})
}

func (r *Repository) SetCurrentRevision(ctx context.Context, nodeID, revisionID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&nodeRow{}).
		Where("id = ? AND current_revision_id IS NULL", nodeID).
		Where("EXISTS (SELECT 1 FROM filesystem.file_revisions WHERE id = ? AND node_id = ?)", revisionID, nodeID).
		Update("current_revision_id", revisionID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var node nodeRow
		if errors.Is(r.db.WithContext(ctx).Where("id = ?", nodeID).First(&node).Error, gorm.ErrRecordNotFound) {
			return domain.ErrNodeNotFound
		}
		return domain.ErrNodeNotFound
	}
	return nil
}

func (r *Repository) MoveNode(ctx context.Context, nodeID uuid.UUID, parentID *uuid.UUID, normalizedName string) error {
	result := r.db.WithContext(ctx).Model(&nodeRow{}).Where("id = ?", nodeID).
		Updates(map[string]any{"parent_id": parentID, "normalized_name": strings.ToLower(strings.TrimSpace(normalizedName)), "name": normalizedName, "updated_at": time.Now().UTC()})
	if isUniqueViolation(result.Error) {
		return domain.ErrNodeNameConflict
	}
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrNodeNotFound
	}
	return nil
}

func (r *Repository) TrashNode(ctx context.Context, nodeID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&nodeRow{}).Where("id = ?", nodeID).Update("state", model.NodeStateTrashed)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrNodeNotFound
	}
	return nil
}

func nodeFromRow(row nodeRow) *model.Node {
	return &model.Node{ID: row.ID, NamespaceID: row.NamespaceID, ParentID: row.ParentID, Type: row.Type, Name: row.Name, NormalizedName: row.NormalizedName, CurrentRevisionID: row.CurrentRevisionID, State: row.State, OwnerID: row.OwnerID}
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") || strings.Contains(msg, "23505")
}

var _ repository.FilesystemRepository = (*Repository)(nil)

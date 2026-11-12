package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	return wrapPersistence(r.db.WithContext(ctx).Create(&namespaceRow{
		ID: namespace.ID, OrganizationID: namespace.OrganizationID, WorkspaceID: namespace.WorkspaceID,
		RootNodeID: namespace.RootNodeID, Status: status, CreatedAt: now, UpdatedAt: now,
	}).Error)
}

func (r *Repository) GetNamespaceByWorkspace(ctx context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
	var row namespaceRow
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNamespaceNotFound
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	return &model.Namespace{ID: row.ID, OrganizationID: row.OrganizationID, WorkspaceID: row.WorkspaceID, RootNodeID: row.RootNodeID, Status: row.Status}, nil
}

func (r *Repository) CreateNode(ctx context.Context, node *model.Node) error {
	db := r.db.WithContext(ctx)
	if node.ParentID != nil {
		var parent nodeRow
		err := db.Where("id = ?", *node.ParentID).First(&parent).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrParentNotFound
		}
		if err != nil {
			return wrapPersistence(err)
		}
		if parent.NamespaceID != node.NamespaceID {
			return domain.ErrNamespaceMismatch
		}
		if parent.Type != model.NodeTypeDirectory {
			return domain.ErrParentNotDirectory
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
		return wrapPersistence(err)
	}
	now := time.Now().UTC()
	err := db.Create(&nodeRow{
		ID: node.ID, NamespaceID: node.NamespaceID, ParentID: node.ParentID, Type: node.Type,
		Name: node.Name, NormalizedName: node.NormalizedName, CurrentRevisionID: node.CurrentRevisionID,
		State: node.State, OwnerID: node.OwnerID, CreatedAt: now, UpdatedAt: now,
	}).Error
	if conflict := mapUniqueConflict(err); conflict != nil {
		return conflict
	}
	return wrapPersistence(err)
}

func (r *Repository) GetNode(ctx context.Context, namespaceID, nodeID uuid.UUID) (*model.Node, error) {
	var row nodeRow
	err := r.db.WithContext(ctx).Where("namespace_id = ? AND id = ?", namespaceID, nodeID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNodeNotFound
	}
	if err != nil {
		return nil, wrapPersistence(err)
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
		return nil, "", wrapPersistence(err)
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
	if revision.BlobID() != blob.ID {
		return domain.ErrRevisionBlobMismatch
	}
	if replica.BlobID != blob.ID {
		return domain.ErrReplicaBlobMismatch
	}
	var node nodeRow
	nodeErr := r.db.WithContext(ctx).Where("id = ?", revision.NodeID()).First(&node).Error
	if errors.Is(nodeErr, gorm.ErrRecordNotFound) {
		return domain.ErrNodeNotFound
	}
	if nodeErr != nil {
		return wrapPersistence(nodeErr)
	}
	for attempt := 0; attempt < 10; attempt++ {
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			now := time.Now().UTC()
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&blobRow{
				ID: blob.ID, DedupDomain: blob.DedupDomain, SHA256: blob.SHA256, Size: blob.Size,
				EncryptionKeyRef: blob.EncryptionKeyRef, RefCount: blob.RefCount, State: blob.State, CreatedAt: now,
			}).Error; err != nil {
				if conflict := mapUniqueConflict(err); conflict != nil && !errors.Is(conflict, domain.ErrPersistence) {
					return conflict
				}
				return err
			}
			var existingBlob blobRow
			if err := tx.Where("id = ?", blob.ID).First(&existingBlob).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrBlobConflict
			} else if err != nil {
				return err
			} else if !sameBlob(existingBlob, blob) {
				return domain.ErrBlobConflict
			}

			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&revisionRow{
				ID: revision.ID(), NodeID: revision.NodeID(), RevisionNo: revision.RevisionNo(), BlobID: revision.BlobID(),
				Size: revision.Size(), MIMEType: revision.MIMEType(), CreatedBy: revision.CreatedBy(), CreatedAt: now,
			}).Error; err != nil {
				if conflict := mapUniqueConflict(err); conflict != nil && !errors.Is(conflict, domain.ErrPersistence) {
					return conflict
				}
				return err
			}
			var existingRevision revisionRow
			if err := tx.Where("id = ?", revision.ID()).First(&existingRevision).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrRevisionConflict
			} else if err != nil {
				return err
			} else if !sameRevision(existingRevision, revision) {
				return domain.ErrRevisionConflict
			}

			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&replicaRow{
				ID: replica.ID, BlobID: replica.BlobID, ProviderID: replica.ProviderID, ObjectKey: replica.ObjectKey,
				Checksum: replica.Checksum, State: replica.State, CreatedAt: now,
			}).Error; err != nil {
				if conflict := mapUniqueConflict(err); conflict != nil && !errors.Is(conflict, domain.ErrPersistence) {
					return conflict
				}
				return err
			}
			var existingReplica replicaRow
			if err := tx.Where("id = ?", replica.ID).First(&existingReplica).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrReplicaConflict
			} else if err != nil {
				return err
			} else if !sameReplica(existingReplica, replica) {
				return domain.ErrReplicaConflict
			}
			return nil
		})
		if err == nil {
			return nil
		}
		if r.db.Dialector.Name() != "sqlite" || !isSQLiteBusy(err) || attempt == 9 {
			return wrapPersistence(err)
		}
	}
	return domain.ErrPersistence
}

func (r *Repository) SetCurrentRevision(ctx context.Context, nodeID, revisionID uuid.UUID) error {
	return wrapPersistence(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var node nodeRow
		if err := tx.Where("id = ?", nodeID).First(&node).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNodeNotFound
		} else if err != nil {
			return wrapPersistence(err)
		}
		var revision revisionRow
		if err := tx.Where("id = ?", revisionID).First(&revision).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrRevisionNotFound
		} else if err != nil {
			return wrapPersistence(err)
		}
		if revision.NodeID != nodeID {
			return domain.ErrRevisionNodeMismatch
		}
		if err := tx.Model(&nodeRow{}).Where("id = ?", nodeID).Update("current_revision_id", revisionID).Error; err != nil {
			return wrapPersistence(err)
		}
		return nil
	}))
}

func (r *Repository) MoveNode(ctx context.Context, nodeID uuid.UUID, parentID *uuid.UUID, normalizedName string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var node nodeRow
		if err := tx.Where("id = ?", nodeID).First(&node).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNodeNotFound
		} else if err != nil {
			return wrapPersistence(err)
		}
		if parentID != nil {
			var parent nodeRow
			if err := tx.Where("id = ?", *parentID).First(&parent).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrParentNotFound
			} else if err != nil {
				return wrapPersistence(err)
			}
			if parent.NamespaceID != node.NamespaceID {
				return domain.ErrNamespaceMismatch
			}
			if parent.Type != model.NodeTypeDirectory {
				return domain.ErrParentNotDirectory
			}
		}
		canonical := strings.TrimSpace(norm.NFC.String(normalizedName))
		if canonical == "" || strings.ContainsAny(canonical, `/\`) {
			return model.ErrInvalidNodeName
		}
		result := tx.Model(&nodeRow{}).Where("id = ?", nodeID).
			Updates(map[string]any{"parent_id": parentID, "normalized_name": strings.ToLower(canonical), "name": canonical, "updated_at": time.Now().UTC()})
		if conflict := mapUniqueConflict(result.Error); conflict != nil {
			return conflict
		}
		return wrapPersistence(result.Error)
	})
}

func (r *Repository) TrashNode(ctx context.Context, nodeID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&nodeRow{}).Where("id = ?", nodeID).Update("state", model.NodeStateTrashed)
	if result.Error != nil {
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNodeNotFound
	}
	return nil
}

func nodeFromRow(row nodeRow) *model.Node {
	return &model.Node{ID: row.ID, NamespaceID: row.NamespaceID, ParentID: row.ParentID, Type: row.Type, Name: row.Name, NormalizedName: row.NormalizedName, CurrentRevisionID: row.CurrentRevisionID, State: row.State, OwnerID: row.OwnerID}
}

func mapUniqueConflict(err error) error {
	if err == nil {
		return nil
	}
	constraint := ""
	message := strings.ToLower(err.Error())
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code != "23505" {
			return domain.ErrPersistence
		}
		constraint = strings.ToLower(pgErr.ConstraintName)
		switch constraint {
		case "nodes_namespace_id_parent_id_normalized_name_key", "nodes_namespace_id_normalized_name_key":
			return domain.ErrNodeNameConflict
		case "nodes_pkey":
			return domain.ErrNodeIDConflict
		case "file_revisions_pkey", "file_revisions_node_id_revision_no_key":
			return domain.ErrRevisionConflict
		case "blobs_pkey", "blobs_dedup_domain_sha256_size_key":
			return domain.ErrBlobConflict
		case "storage_replicas_pkey", "storage_replicas_blob_id_provider_id_object_key_key":
			return domain.ErrReplicaConflict
		default:
			return domain.ErrPersistence
		}
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.ErrPersistence
	}
	if !strings.Contains(message, "unique constraint failed") {
		return nil
	}
	switch {
	case strings.Contains(message, "ux_filesystem_nodes_root_name"),
		strings.Contains(message, "ux_filesystem_nodes_child_name"),
		strings.Contains(message, "nodes.namespace_id, nodes.normalized_name"),
		strings.Contains(message, "nodes.namespace_id, filesystem.nodes.normalized_name"),
		strings.Contains(message, "nodes.namespace_id, nodes.parent_id, nodes.normalized_name"):
		return domain.ErrNodeNameConflict
	case strings.Contains(message, "nodes.id"):
		return domain.ErrNodeIDConflict
	case strings.Contains(message, "file_revisions"):
		return domain.ErrRevisionConflict
	case strings.Contains(message, "blobs"):
		return domain.ErrBlobConflict
	case strings.Contains(message, "storage_replicas"):
		return domain.ErrReplicaConflict
	default:
		return domain.ErrPersistence
	}
}

func wrapPersistence(err error) error {
	if err == nil {
		return nil
	}
	if isDomainError(err) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrPersistence
	}
	if conflict := mapUniqueConflict(err); conflict != nil {
		return conflict
	}
	return fmt.Errorf("%w: operation failed", domain.ErrPersistence)
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") ||
		strings.Contains(message, "database table is locked") ||
		strings.Contains(message, "busy")
}

func isDomainError(err error) bool {
	for _, sentinel := range []error{
		domain.ErrNodeNameConflict, domain.ErrNodeNotFound, domain.ErrNodeIDConflict,
		domain.ErrNamespaceNotFound, domain.ErrRevisionNotFound, domain.ErrRevisionConflict,
		domain.ErrRevisionNodeMismatch, domain.ErrParentNotFound, domain.ErrParentNotDirectory,
		domain.ErrNamespaceMismatch, domain.ErrBlobConflict, domain.ErrReplicaConflict,
		domain.ErrRevisionBlobMismatch, domain.ErrReplicaBlobMismatch,
		domain.ErrPersistence, model.ErrInvalidNodeName,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

func sameBlob(row blobRow, blob *model.Blob) bool {
	return row.ID == blob.ID && row.DedupDomain == blob.DedupDomain && row.SHA256 == blob.SHA256 &&
		row.Size == blob.Size && row.EncryptionKeyRef == blob.EncryptionKeyRef && row.RefCount == blob.RefCount && row.State == blob.State
}

func sameRevision(row revisionRow, revision *model.FileRevision) bool {
	return row.ID == revision.ID() && row.NodeID == revision.NodeID() && row.RevisionNo == revision.RevisionNo() &&
		row.BlobID == revision.BlobID() && row.Size == revision.Size() && row.MIMEType == revision.MIMEType() && row.CreatedBy == revision.CreatedBy()
}

func sameReplica(row replicaRow, replica *model.StorageReplica) bool {
	return row.ID == replica.ID && row.BlobID == replica.BlobID && row.ProviderID == replica.ProviderID &&
		row.ObjectKey == replica.ObjectKey && row.Checksum == replica.Checksum && row.State == replica.State
}

var _ repository.FilesystemRepository = (*Repository)(nil)

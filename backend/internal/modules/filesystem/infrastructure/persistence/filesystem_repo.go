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
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) repository.FilesystemRepository { return &Repository{db: db} }

func (r *Repository) CreateNamespace(ctx context.Context, namespace *model.Namespace) error {
	now := time.Now().UTC()
	status := namespace.Status
	if status == "" {
		status = "ACTIVE"
	}
	db := postgres.DB(ctx, r.db)
	return wrapPersistence(db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&namespaceRow{
			ID: namespace.ID, OrganizationID: namespace.OrganizationID, WorkspaceID: namespace.WorkspaceID,
			RootNodeID: namespace.RootNodeID, Status: status, CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&quotaAccountRow{ID: uuid.New(), OrganizationID: namespace.OrganizationID, WorkspaceID: namespace.WorkspaceID, UpdatedAt: now}).Error
	}))
}

func (r *Repository) GetNamespaceByWorkspace(ctx context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
	var row namespaceRow
	err := postgres.DB(ctx, r.db).Where("workspace_id = ?", workspaceID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNamespaceNotFound
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	return &model.Namespace{ID: row.ID, OrganizationID: row.OrganizationID, WorkspaceID: row.WorkspaceID, RootNodeID: row.RootNodeID, Status: row.Status}, nil
}

func (r *Repository) SetNamespaceRoot(ctx context.Context, workspaceID, rootNodeID uuid.UUID) error {
	result := postgres.DB(ctx, r.db).Model(&namespaceRow{}).
		Where("workspace_id = ?", workspaceID).
		Updates(map[string]any{"root_node_id": rootNodeID, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNamespaceNotFound
	}
	return nil
}

func (r *Repository) CreateNode(ctx context.Context, node *model.Node) error {
	db := postgres.DB(ctx, r.db)
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
		if parent.State != model.NodeStateActive {
			return domain.ErrParentNotFound
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
	node, err := r.GetNodeIncludingTrashed(ctx, namespaceID, nodeID)
	if err != nil {
		return nil, err
	}
	if node.State != model.NodeStateActive {
		return nil, domain.ErrNodeNotFound
	}
	current := node
	for current.ParentID != nil {
		parent, parentErr := r.GetNodeIncludingTrashed(ctx, namespaceID, *current.ParentID)
		if parentErr != nil || parent.State != model.NodeStateActive {
			return nil, domain.ErrNodeNotFound
		}
		current = parent
	}
	return node, nil
}

func (r *Repository) GetNodeIncludingTrashed(ctx context.Context, namespaceID, nodeID uuid.UUID) (*model.Node, error) {
	var row nodeRow
	err := postgres.DB(ctx, r.db).Where("namespace_id = ? AND id = ?", namespaceID, nodeID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNodeNotFound
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	return nodeFromRow(row), nil
}

func (r *Repository) ListChildren(ctx context.Context, namespaceID uuid.UUID, parentID *uuid.UUID, limit int, cursor string) ([]model.Node, string, error) {
	if parentID != nil {
		if _, err := r.GetNode(ctx, namespaceID, *parentID); err != nil {
			return nil, "", err
		}
	}
	query := postgres.DB(ctx, r.db).Where("namespace_id = ? AND state = ?", namespaceID, model.NodeStateActive)
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

func (r *Repository) NextRevisionNumber(ctx context.Context, nodeID uuid.UUID) (int, error) {
	tx, err := postgres.RequireTx(ctx)
	if err != nil {
		return 0, domain.ErrPersistence
	}
	var node nodeRow
	if tx.Dialector.Name() == "postgres" {
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", nodeID).First(&node).Error
	} else {
		result := tx.Exec("UPDATE filesystem.nodes SET updated_at = updated_at WHERE id = ?", nodeID)
		err = result.Error
		if isSQLiteBusy(err) {
			return 0, domain.ErrTransactionRetry
		}
		if err == nil && result.RowsAffected == 0 {
			err = gorm.ErrRecordNotFound
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, domain.ErrNodeNotFound
	}
	if err != nil {
		return 0, wrapPersistence(err)
	}
	var maxRevision int
	if err := tx.Table("filesystem.file_revisions").
		Select("COALESCE(MAX(revision_no), 0)").
		Where("node_id = ?", nodeID).
		Scan(&maxRevision).Error; err != nil {
		return 0, wrapPersistence(err)
	}
	return maxRevision + 1, nil
}

func (r *Repository) SaveRevision(ctx context.Context, revision *model.FileRevision, blob *model.Blob, replica *model.StorageReplica) error {
	if revision.BlobID() != blob.ID {
		return domain.ErrRevisionBlobMismatch
	}
	if replica != nil && replica.BlobID != blob.ID {
		return domain.ErrReplicaBlobMismatch
	}
	if replica != nil && replica.State == model.ReplicaStateAvailable && replica.VerifiedAt == nil {
		verifiedAt := time.Now().UTC()
		replica.VerifiedAt = &verifiedAt
	}
	var node nodeRow
	nodeErr := postgres.DB(ctx, r.db).Where("id = ?", revision.NodeID()).First(&node).Error
	if errors.Is(nodeErr, gorm.ErrRecordNotFound) {
		return domain.ErrNodeNotFound
	}
	if nodeErr != nil {
		return wrapPersistence(nodeErr)
	}
	save := func(tx *gorm.DB) error {
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
		if err := tx.Where("dedup_domain = ? AND sha256 = ? AND size = ?", blob.DedupDomain, blob.SHA256, blob.Size).First(&existingBlob).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrBlobConflict
		} else if err != nil {
			return err
		} else if existingBlob.ID != blob.ID {
			return domain.ErrRevisionBlobMismatch
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

		if replica == nil {
			return nil
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&replicaRow{
			ID: replica.ID, BlobID: replica.BlobID, ProviderID: replica.ProviderID, ObjectKey: replica.ObjectKey,
			Checksum: replica.Checksum, State: replica.State, VerifiedAt: replica.VerifiedAt, CreatedAt: now,
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
	}
	if _, err := postgres.RequireTx(ctx); err == nil {
		return wrapPersistence(save(postgres.DB(ctx, r.db)))
	}
	for attempt := 0; attempt < 10; attempt++ {
		err := r.db.WithContext(ctx).Transaction(save)
		if err == nil {
			return nil
		}
		if r.db.Dialector.Name() != "sqlite" || !isSQLiteBusy(err) || attempt == 9 {
			return wrapPersistence(err)
		}
	}
	return domain.ErrPersistence
}

func (r *Repository) AcquireBlob(ctx context.Context, dedupDomain uuid.UUID, checksum string, size int64) (*model.Blob, error) {
	db := postgres.DB(ctx, r.db)
	candidate := blobRow{
		ID: uuid.New(), DedupDomain: dedupDomain, SHA256: strings.ToLower(checksum), Size: size,
		RefCount: 0, State: model.BlobStateAvailable, CreatedAt: time.Now().UTC(),
	}
	if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{
		{Name: "dedup_domain"}, {Name: "sha256"}, {Name: "size"},
	}, DoNothing: true}).Create(&candidate).Error; err != nil {
		if db.Dialector.Name() == "sqlite" && isSQLiteBusy(err) {
			return nil, domain.ErrTransactionRetry
		}
		return nil, wrapPersistence(err)
	}
	var canonical blobRow
	if err := db.Where("dedup_domain = ? AND sha256 = ? AND size = ?", dedupDomain, strings.ToLower(checksum), size).First(&canonical).Error; err != nil {
		return nil, wrapPersistence(err)
	}
	result := db.Model(&blobRow{}).Where("id = ?", canonical.ID).UpdateColumn("ref_count", gorm.Expr("ref_count + 1"))
	if result.Error != nil {
		return nil, wrapPersistence(result.Error)
	}
	if result.RowsAffected != 1 {
		return nil, domain.ErrBlobConflict
	}
	canonical.RefCount++
	return &model.Blob{ID: canonical.ID, DedupDomain: canonical.DedupDomain, SHA256: canonical.SHA256, Size: canonical.Size, EncryptionKeyRef: canonical.EncryptionKeyRef, RefCount: canonical.RefCount, State: canonical.State}, nil
}

func (r *Repository) GetRevision(ctx context.Context, nodeID, revisionID uuid.UUID) (*model.FileRevision, error) {
	var row revisionRow
	err := postgres.DB(ctx, r.db).Where("node_id = ? AND id = ?", nodeID, revisionID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrRevisionNotFound
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	revision, err := model.NewRevisionWithID(row.ID, row.NodeID, row.RevisionNo, row.BlobID, row.Size, row.MIMEType, row.CreatedBy)
	if err != nil {
		return nil, domain.ErrPersistence
	}
	return revision, nil
}

func (r *Repository) ListRevisions(ctx context.Context, nodeID, namespaceID uuid.UUID, limit int, cursor string) ([]model.FileRevision, string, error) {
	if _, err := r.GetNode(ctx, namespaceID, nodeID); err != nil {
		return nil, "", err
	}
	query := postgres.DB(ctx, r.db).Table("filesystem.file_revisions").
		Joins("JOIN filesystem.nodes ON filesystem.nodes.id = filesystem.file_revisions.node_id").
		Where("filesystem.file_revisions.node_id = ? AND filesystem.nodes.namespace_id = ?", nodeID, namespaceID)
	if cursor != "" {
		var boundary revisionRow
		if err := postgres.DB(ctx, r.db).Table("filesystem.file_revisions").
			Joins("JOIN filesystem.nodes ON filesystem.nodes.id = filesystem.file_revisions.node_id").
			Where("filesystem.file_revisions.node_id = ? AND filesystem.file_revisions.id = ? AND filesystem.nodes.namespace_id = ?", nodeID, cursor, namespaceID).
			First(&boundary).Error; err != nil {
			return nil, "", domain.ErrRevisionNotFound
		}
		query = query.Where("(filesystem.file_revisions.revision_no < ?) OR (filesystem.file_revisions.revision_no = ? AND filesystem.file_revisions.id < ?)", boundary.RevisionNo, boundary.RevisionNo, boundary.ID)
	}
	if limit <= 0 {
		limit = 100
	}
	var rows []revisionRow
	if err := query.Order("filesystem.file_revisions.revision_no DESC, filesystem.file_revisions.id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", wrapPersistence(err)
	}
	next := ""
	if len(rows) > limit {
		next = rows[limit-1].ID.String()
		rows = rows[:limit]
	}
	out := make([]model.FileRevision, 0, len(rows))
	for _, row := range rows {
		revision, err := model.NewRevisionWithID(row.ID, row.NodeID, row.RevisionNo, row.BlobID, row.Size, row.MIMEType, row.CreatedBy)
		if err != nil {
			return nil, "", domain.ErrPersistence
		}
		out = append(out, *revision)
	}
	return out, next, nil
}

func (r *Repository) IncrementBlobRef(ctx context.Context, blobID uuid.UUID) error {
	result := postgres.DB(ctx, r.db).Model(&blobRow{}).Where("id = ?", blobID).UpdateColumn("ref_count", gorm.Expr("ref_count + 1"))
	if result.Error != nil {
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrBlobConflict
	}
	return nil
}

func (r *Repository) GetBlob(ctx context.Context, blobID uuid.UUID) (*model.Blob, error) {
	var row blobRow
	err := postgres.DB(ctx, r.db).Where("id = ?", blobID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrBlobConflict
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	return &model.Blob{ID: row.ID, DedupDomain: row.DedupDomain, SHA256: row.SHA256, Size: row.Size, EncryptionKeyRef: row.EncryptionKeyRef, RefCount: row.RefCount, State: row.State}, nil
}

func (r *Repository) GetAvailableReplica(ctx context.Context, blobID, providerID uuid.UUID) (*model.StorageReplica, error) {
	var row replicaRow
	query := postgres.DB(ctx, r.db).Where("blob_id = ? AND state = ?", blobID, model.ReplicaStateAvailable)
	if providerID != uuid.Nil {
		query = query.Where("provider_id = ?", providerID)
	}
	if err := query.Order("id ASC").First(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrReplicaConflict
	} else if err != nil {
		return nil, wrapPersistence(err)
	}
	return &model.StorageReplica{ID: row.ID, BlobID: row.BlobID, ProviderID: row.ProviderID, ObjectKey: row.ObjectKey, Checksum: row.Checksum, State: row.State, VerifiedAt: row.VerifiedAt}, nil
}

func (r *Repository) ListResourceGrants(ctx context.Context, nodeID, granteeID uuid.UUID, permission model.ResourcePermission, at time.Time) ([]model.ResourceGrant, error) {
	var rows []resourceGrantRow
	err := postgres.DB(ctx, r.db).
		Where("node_id = ? AND grantee_id = ? AND permission = ? AND (expires_at IS NULL OR expires_at > ?)", nodeID, granteeID, permission, at).
		Find(&rows).Error
	if err != nil {
		return nil, wrapPersistence(err)
	}
	grants := make([]model.ResourceGrant, len(rows))
	for index, row := range rows {
		grants[index] = model.ResourceGrant{ID: row.ID, NodeID: row.NodeID, GranteeID: row.GranteeID, Permission: row.Permission, Effect: row.Effect, ExpiresAt: row.ExpiresAt}
	}
	return grants, nil
}

func (r *Repository) CreateResourceGrant(ctx context.Context, grant model.ResourceGrant) error {
	return wrapPersistence(postgres.DB(ctx, r.db).Create(&resourceGrantRow{
		ID: grant.ID, NodeID: grant.NodeID, GranteeID: grant.GranteeID, Permission: grant.Permission,
		Effect: grant.Effect, ExpiresAt: grant.ExpiresAt, CreatedAt: time.Now().UTC(),
	}).Error)
}

func (r *Repository) RevokeResourceGrant(ctx context.Context, nodeID, grantID uuid.UUID) error {
	result := postgres.DB(ctx, r.db).Where("id = ? AND node_id = ?", grantID, nodeID).Delete(&resourceGrantRow{})
	if result.Error != nil {
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrGrantNotFound
	}
	return nil
}

func (r *Repository) ListNodeResourceGrants(ctx context.Context, nodeID uuid.UUID) ([]model.ResourceGrant, error) {
	var rows []resourceGrantRow
	if err := postgres.DB(ctx, r.db).Where("node_id = ?", nodeID).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, wrapPersistence(err)
	}
	result := make([]model.ResourceGrant, len(rows))
	for i, row := range rows {
		result[i] = model.ResourceGrant{ID: row.ID, NodeID: row.NodeID, GranteeID: row.GranteeID, Permission: row.Permission, Effect: row.Effect, ExpiresAt: row.ExpiresAt}
	}
	return result, nil
}

func (r *Repository) ApplyQuotaUsage(ctx context.Context, workspaceID uuid.UUID, bytesDelta, filesDelta int64) error {
	if bytesDelta < 0 || filesDelta < 0 {
		return domain.ErrPersistence
	}
	db := postgres.DB(ctx, r.db)
	result := db.Model(&quotaAccountRow{}).
		Where("workspace_id = ? AND (max_bytes IS NULL OR (max_bytes >= used_bytes AND max_bytes - used_bytes >= reserved_bytes AND max_bytes - used_bytes - reserved_bytes >= ?)) AND (max_files IS NULL OR (max_files >= used_files AND max_files - used_files >= reserved_files AND max_files - used_files - reserved_files >= ?))", workspaceID, bytesDelta, filesDelta).
		Updates(map[string]any{
			"used_bytes": gorm.Expr("used_bytes + ?", bytesDelta),
			"used_files": gorm.Expr("used_files + ?", filesDelta),
			"version":    gorm.Expr("version + 1"),
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		var count int64
		if err := db.Model(&quotaAccountRow{}).Where("workspace_id = ?", workspaceID).Count(&count).Error; err != nil {
			return wrapPersistence(err)
		}
		if count == 0 {
			return domain.ErrNamespaceNotFound
		}
		return domain.ErrQuotaExceeded
	}
	return nil
}

func (r *Repository) RenameNode(ctx context.Context, nodeID uuid.UUID, name string) error {
	var node nodeRow
	db := postgres.DB(ctx, r.db)
	if err := db.Where("id = ?", nodeID).First(&node).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNodeNotFound
	} else if err != nil {
		return wrapPersistence(err)
	}
	canonical := strings.TrimSpace(norm.NFC.String(name))
	if canonical == "" || strings.ContainsAny(canonical, `/\`) {
		return model.ErrInvalidNodeName
	}
	result := db.Model(&nodeRow{}).Where("id = ?", nodeID).Updates(map[string]any{"name": canonical, "normalized_name": strings.ToLower(canonical), "updated_at": time.Now().UTC()})
	if conflict := mapUniqueConflict(result.Error); conflict != nil {
		return conflict
	}
	return wrapPersistence(result.Error)
}

func (r *Repository) SetCurrentRevision(ctx context.Context, nodeID, revisionID uuid.UUID) error {
	return wrapPersistence(postgres.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
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

func (r *Repository) MoveNodeSafely(ctx context.Context, namespaceID, nodeID uuid.UUID, parentID *uuid.UUID, name string) error {
	tx, err := postgres.RequireTx(ctx)
	if err != nil {
		return domain.ErrPersistence
	}
	var namespace namespaceRow
	if tx.Dialector.Name() == "postgres" {
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", namespaceID).First(&namespace).Error
	} else {
		result := tx.Exec("UPDATE filesystem.namespaces SET updated_at = updated_at WHERE id = ?", namespaceID)
		err = result.Error
		if isSQLiteBusy(err) {
			return domain.ErrTransactionRetry
		}
		if err == nil && result.RowsAffected == 0 {
			err = gorm.ErrRecordNotFound
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNamespaceNotFound
	}
	if err != nil {
		if tx.Dialector.Name() == "sqlite" && isSQLiteBusy(err) {
			return domain.ErrTransactionRetry
		}
		return wrapPersistence(err)
	}
	var node nodeRow
	if err := tx.Where("namespace_id = ? AND id = ?", namespaceID, nodeID).First(&node).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNodeNotFound
	} else if err != nil {
		if tx.Dialector.Name() == "sqlite" && isSQLiteBusy(err) {
			return domain.ErrTransactionRetry
		}
		return wrapPersistence(err)
	}
	if parentID != nil {
		visited := map[uuid.UUID]struct{}{nodeID: {}}
		current := *parentID
		for depth := 0; depth < 10000; depth++ {
			if _, exists := visited[current]; exists {
				return domain.ErrDirectoryCycle
			}
			visited[current] = struct{}{}
			var parent nodeRow
			if err := tx.Where("namespace_id = ? AND id = ?", namespaceID, current).First(&parent).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrParentNotFound
			} else if err != nil {
				if tx.Dialector.Name() == "sqlite" && isSQLiteBusy(err) {
					return domain.ErrTransactionRetry
				}
				return wrapPersistence(err)
			}
			if parent.Type != model.NodeTypeDirectory {
				return domain.ErrParentNotDirectory
			}
			if parent.ParentID == nil {
				break
			}
			current = *parent.ParentID
			if depth == 9999 {
				return domain.ErrDirectoryCycle
			}
		}
	}
	canonical := strings.TrimSpace(norm.NFC.String(name))
	if canonical == "" || strings.ContainsAny(canonical, `/\`) {
		return model.ErrInvalidNodeName
	}
	result := tx.Model(&nodeRow{}).Where("namespace_id = ? AND id = ?", namespaceID, nodeID).
		Updates(map[string]any{"parent_id": parentID, "normalized_name": strings.ToLower(canonical), "name": canonical, "updated_at": time.Now().UTC()})
	if tx.Dialector.Name() == "sqlite" && isSQLiteBusy(result.Error) {
		return domain.ErrTransactionRetry
	}
	if conflict := mapUniqueConflict(result.Error); conflict != nil {
		return conflict
	}
	return wrapPersistence(result.Error)
}

func (r *Repository) TrashNode(ctx context.Context, nodeID uuid.UUID) error {
	result := postgres.DB(ctx, r.db).Model(&nodeRow{}).Where("id = ?", nodeID).Update("state", model.NodeStateTrashed)
	if result.Error != nil {
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNodeNotFound
	}
	return nil
}

func (r *Repository) GetCommand(ctx context.Context, commandID uuid.UUID) (*repository.CommandDedup, error) {
	var row commandDedupRow
	err := postgres.DB(ctx, r.db).Where("command_id = ?", commandID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrCommandNotFound
	}
	if err != nil {
		return nil, domain.ErrPersistence
	}
	return commandFromRow(row), nil
}

func (r *Repository) ClaimCommand(ctx context.Context, command repository.CommandDedup) (*repository.CommandDedup, error) {
	if _, err := postgres.RequireTx(ctx); err != nil {
		return nil, domain.ErrPersistence
	}
	now := time.Now().UTC()
	row := commandDedupRow{CommandID: command.CommandID, WorkspaceID: command.WorkspaceID, Operation: command.Operation, RequestHash: command.RequestHash, NodeID: command.NodeID, RevisionID: command.RevisionID, State: "IN_PROGRESS", CreatedAt: now, UpdatedAt: now}
	db := postgres.DB(ctx, r.db)
	result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		if db.Dialector.Name() == "sqlite" && isSQLiteBusy(result.Error) {
			return nil, domain.ErrTransactionRetry
		}
		return nil, wrapPersistence(result.Error)
	}
	if result.RowsAffected == 1 {
		return commandFromRow(row), nil
	}
	var existing commandDedupRow
	if readErr := db.Where("command_id = ?", command.CommandID).First(&existing).Error; readErr != nil {
		return nil, wrapPersistence(readErr)
	}
	if existing.RequestHash != command.RequestHash || existing.WorkspaceID != command.WorkspaceID || existing.Operation != command.Operation {
		return nil, domain.ErrIdempotencyConflict
	}
	if existing.State != "COMPLETED" {
		return nil, domain.ErrCommandInProgress
	}
	return commandFromRow(existing), nil
}

func (r *Repository) CompleteCommand(ctx context.Context, commandID uuid.UUID, nodeID, revisionID *uuid.UUID) error {
	result := postgres.DB(ctx, r.db).Model(&commandDedupRow{}).Where("command_id = ? AND state = ?", commandID, "IN_PROGRESS").Updates(map[string]any{"node_id": nodeID, "revision_id": revisionID, "state": "COMPLETED", "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return domain.ErrPersistence
	}
	if result.RowsAffected == 0 {
		return domain.ErrCommandInProgress
	}
	return nil
}

func commandFromRow(row commandDedupRow) *repository.CommandDedup {
	return &repository.CommandDedup{CommandID: row.CommandID, WorkspaceID: row.WorkspaceID, Operation: row.Operation, RequestHash: row.RequestHash, NodeID: row.NodeID, RevisionID: row.RevisionID, State: row.State}
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
	case strings.Contains(message, "file_revisions.id"),
		strings.Contains(message, "file_revisions.node_id, file_revisions.revision_no"),
		strings.Contains(message, "file_revisions.node_id, filesystem.file_revisions.revision_no"):
		return domain.ErrRevisionConflict
	case strings.Contains(message, "blobs.id"),
		strings.Contains(message, "blobs.dedup_domain, blobs.sha256, blobs.size"),
		strings.Contains(message, "blobs.dedup_domain, filesystem.blobs.sha256, filesystem.blobs.size"):
		return domain.ErrBlobConflict
	case strings.Contains(message, "storage_replicas.id"),
		strings.Contains(message, "storage_replicas.blob_id, storage_replicas.provider_id, storage_replicas.object_key"),
		strings.Contains(message, "storage_replicas.blob_id, filesystem.storage_replicas.provider_id, filesystem.storage_replicas.object_key"):
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
		domain.ErrPersistence, domain.ErrQuotaExceeded, domain.ErrDirectoryCycle,
		domain.ErrIdempotencyConflict, domain.ErrOutcomeUnknown, domain.ErrCommandInProgress, model.ErrInvalidNodeName,
		domain.ErrCommandNotFound, domain.ErrTransactionRetry,
		domain.ErrTransactionExhausted, domain.ErrUnsupportedNodeType, domain.ErrRootMutation,
		domain.ErrResourceDenied,
		domain.ErrGrantNotFound, domain.ErrNodeStateConflict,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

func sameBlob(row blobRow, blob *model.Blob) bool {
	return row.ID == blob.ID && row.DedupDomain == blob.DedupDomain && row.SHA256 == blob.SHA256 &&
		row.Size == blob.Size && row.EncryptionKeyRef == blob.EncryptionKeyRef && row.State == blob.State
}

func sameRevision(row revisionRow, revision *model.FileRevision) bool {
	return row.ID == revision.ID() && row.NodeID == revision.NodeID() && row.RevisionNo == revision.RevisionNo() &&
		row.BlobID == revision.BlobID() && row.Size == revision.Size() && row.MIMEType == revision.MIMEType() && row.CreatedBy == revision.CreatedBy()
}

func sameReplica(row replicaRow, replica *model.StorageReplica) bool {
	return row.ID == replica.ID && row.BlobID == replica.BlobID && row.ProviderID == replica.ProviderID &&
		row.ObjectKey == replica.ObjectKey && row.Checksum == replica.Checksum && row.State == replica.State &&
		equalTimePointers(row.VerifiedAt, replica.VerifiedAt)
}

func equalTimePointers(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func (r *Repository) CreateTransfer(context.Context, *model.TransferSession) error {
	return domain.ErrPersistence
}

func (r *Repository) GetTransferByID(context.Context, uuid.UUID) (*model.TransferSession, error) {
	return nil, domain.ErrPersistence
}

func (r *Repository) GetTransferByIdempotency(context.Context, uuid.UUID, string) (*model.TransferSession, error) {
	return nil, domain.ErrPersistence
}

func (r *Repository) CASTransferStatus(context.Context, uuid.UUID, model.TransferStatus, model.TransferStatus, int64) error {
	return domain.ErrPersistence
}

func (r *Repository) LockTransfer(context.Context, uuid.UUID) (*model.TransferSession, error) {
	return nil, domain.ErrPersistence
}

func (r *Repository) ListTransferParts(context.Context, uuid.UUID) ([]model.TransferPart, error) {
	return nil, domain.ErrPersistence
}

func (r *Repository) UpsertTransferPart(context.Context, *model.TransferPart) error {
	return domain.ErrPersistence
}

var _ repository.FilesystemRepository = (*Repository)(nil)

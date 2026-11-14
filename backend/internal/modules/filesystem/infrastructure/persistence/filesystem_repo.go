package persistence

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreateNamespace(ctx context.Context, namespace *model.Namespace) error {
	if err := namespace.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	status := namespace.Status
	if status == "" {
		status = "ACTIVE"
	}
	var workspaceID *uuid.UUID
	if namespace.WorkspaceID != uuid.Nil {
		workspaceID = &namespace.WorkspaceID
	}
	var ownerUserID *uuid.UUID
	if namespace.OwnerUserID != uuid.Nil {
		ownerUserID = &namespace.OwnerUserID
	}
	db := postgres.DB(ctx, r.db)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&namespaceRow{
			ID: namespace.ID, NamespaceType: namespace.NamespaceType, OrganizationID: namespace.OrganizationID,
			WorkspaceID: workspaceID, OwnerUserID: ownerUserID, RootNodeID: namespace.RootNodeID,
			Status: status, PolicyVersion: namespace.PolicyVersion, CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			return err
		}
		if namespace.NamespaceType != model.NamespaceTypeWorkspace {
			return nil
		}
		return tx.Create(&quotaAccountRow{ID: uuid.New(), OrganizationID: namespace.OrganizationID, WorkspaceID: namespace.WorkspaceID, UpdatedAt: now}).Error
	})
	if isSQLiteBusy(err) {
		return domain.ErrTransactionRetry
	}
	return wrapPersistence(err)
}

func (r *Repository) CreateNamespaceIfAbsent(ctx context.Context, namespace *model.Namespace) error {
	if err := namespace.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	var workspaceID, ownerUserID *uuid.UUID
	if namespace.WorkspaceID != uuid.Nil {
		workspaceID = &namespace.WorkspaceID
	}
	if namespace.OwnerUserID != uuid.Nil {
		ownerUserID = &namespace.OwnerUserID
	}
	db := postgres.DB(ctx, r.db)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&namespaceRow{
			ID: namespace.ID, NamespaceType: namespace.NamespaceType, OrganizationID: namespace.OrganizationID,
			WorkspaceID: workspaceID, OwnerUserID: ownerUserID, RootNodeID: namespace.RootNodeID,
			Status: "ACTIVE", PolicyVersion: namespace.PolicyVersion, CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			return err
		}
		if namespace.NamespaceType != model.NamespaceTypeWorkspace {
			return nil
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&quotaAccountRow{
			ID: uuid.New(), OrganizationID: namespace.OrganizationID, WorkspaceID: namespace.WorkspaceID, UpdatedAt: now,
		}).Error
	})
	if isSQLiteBusy(err) {
		return domain.ErrTransactionRetry
	}
	return wrapPersistence(err)
}

func (r *Repository) GetNamespaceByWorkspace(ctx context.Context, workspaceID uuid.UUID) (*model.Namespace, error) {
	var row namespaceRow
	err := postgres.DB(ctx, r.db).
		Where("namespace_type = ? AND workspace_id = ?", model.NamespaceTypeWorkspace, workspaceID).
		First(&row).Error
	return namespaceResult(row, err)
}

func (r *Repository) GetNamespaceByOrganization(ctx context.Context, organizationID uuid.UUID) (*model.Namespace, error) {
	var row namespaceRow
	err := postgres.DB(ctx, r.db).
		Where("namespace_type = ? AND organization_id = ?", model.NamespaceTypeOrganization, organizationID).
		First(&row).Error
	return namespaceResult(row, err)
}

func (r *Repository) GetNamespaceByUser(ctx context.Context, organizationID, userID uuid.UUID) (*model.Namespace, error) {
	var row namespaceRow
	err := postgres.DB(ctx, r.db).
		Where("namespace_type = ? AND organization_id = ? AND owner_user_id = ?",
			model.NamespaceTypeUser, organizationID, userID).
		First(&row).Error
	return namespaceResult(row, err)
}

func (r *Repository) ListNamespacesForOrganization(ctx context.Context, organizationID uuid.UUID) ([]*model.Namespace, error) {
	var rows []namespaceRow
	if err := postgres.DB(ctx, r.db).
		Where("organization_id = ?", organizationID).
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, wrapPersistence(err)
	}
	namespaces := make([]*model.Namespace, len(rows))
	for i := range rows {
		namespaces[i] = namespaceFromRow(&rows[i])
	}
	return namespaces, nil
}

func namespaceResult(row namespaceRow, err error) (*model.Namespace, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNamespaceNotFound
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	return namespaceFromRow(&row), nil
}

func (r *Repository) GetNamespace(ctx context.Context, namespaceID uuid.UUID) (*model.Namespace, error) {
	var row namespaceRow
	err := postgres.DB(ctx, r.db).Where("id = ?", namespaceID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNamespaceNotFound
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	return namespaceFromRow(&row), nil
}

func namespaceFromRow(row *namespaceRow) *model.Namespace {
	namespace := &model.Namespace{
		ID: row.ID, NamespaceType: row.NamespaceType, OrganizationID: row.OrganizationID,
		RootNodeID: row.RootNodeID, Status: row.Status, PolicyVersion: row.PolicyVersion,
	}
	if row.WorkspaceID != nil {
		namespace.WorkspaceID = *row.WorkspaceID
	}
	if row.OwnerUserID != nil {
		namespace.OwnerUserID = *row.OwnerUserID
	}
	return namespace
}

func (r *Repository) SetNamespaceRoot(ctx context.Context, workspaceID, rootNodeID uuid.UUID) error {
	result := postgres.DB(ctx, r.db).Model(&namespaceRow{}).
		Where("namespace_type = ? AND workspace_id = ?", model.NamespaceTypeWorkspace, workspaceID).
		Updates(map[string]any{"root_node_id": rootNodeID, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNamespaceNotFound
	}
	return nil
}

func (r *Repository) SetNamespaceRootByID(ctx context.Context, namespaceID, rootNodeID uuid.UUID) error {
	result := postgres.DB(ctx, r.db).Model(&namespaceRow{}).
		Where("id = ?", namespaceID).
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

func (r *Repository) CreateNamespaceRootNodeIfAbsent(ctx context.Context, node *model.Node) error {
	err := postgres.DB(ctx, r.db).Clauses(clause.OnConflict{DoNothing: true}).Create(&nodeRow{
		ID: node.ID, NamespaceID: node.NamespaceID, ParentID: node.ParentID, Type: node.Type,
		Name: node.Name, NormalizedName: node.NormalizedName, CurrentRevisionID: node.CurrentRevisionID,
		State: node.State, OwnerID: node.OwnerID, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}).Error
	if isSQLiteBusy(err) {
		return domain.ErrTransactionRetry
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

func (r *Repository) ListChildren(ctx context.Context, namespaceID uuid.UUID, parentID *uuid.UUID, nodeType *model.NodeType, limit int, cursor string) ([]model.Node, string, error) {
	return r.ListChildrenFiltered(ctx, namespaceID, parentID, nodeType, "", limit, cursor)
}

func (r *Repository) ListChildrenFiltered(ctx context.Context, namespaceID uuid.UUID, parentID *uuid.UUID, nodeType *model.NodeType, search string, limit int, cursor string) ([]model.Node, string, error) {
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
	if nodeType != nil {
		query = query.Where("node_type = ?", *nodeType)
	}
	needle := strings.ToLower(strings.TrimSpace(norm.NFC.String(search)))
	if needle != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(needle)
		query = query.Where(`normalized_name LIKE ? ESCAPE '\'`, "%"+escaped+"%")
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
			Checksum: replica.Checksum, State: replica.State, VerifiedAt: replica.VerifiedAt, DeleteAfter: replica.DeleteAfter,
			CreatedAt: now,
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
	return &model.Blob{
		ID: row.ID, DedupDomain: row.DedupDomain, SHA256: row.SHA256, Size: row.Size,
		EncryptionKeyRef: row.EncryptionKeyRef, RefCount: row.RefCount, State: row.State,
		PreferredProviderID: row.PreferredProviderID,
		TombstonedAt:        row.TombstonedAt, LegalHold: row.LegalHold,
	}, nil
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
	return replicaModelFromRow(row), nil
}

func (r *Repository) GetPreferredReplica(ctx context.Context, blobID uuid.UUID) (*model.StorageReplica, error) {
	blob, err := r.GetBlob(ctx, blobID)
	if err != nil {
		return nil, err
	}
	providerID := uuid.Nil
	if blob.PreferredProviderID != nil {
		providerID = *blob.PreferredProviderID
	}
	return r.GetAvailableReplica(ctx, blobID, providerID)
}

func (r *Repository) GetReplicaByProvider(ctx context.Context, blobID, providerID uuid.UUID) (*model.StorageReplica, error) {
	var row replicaRow
	err := postgres.DB(ctx, r.db).Where("blob_id = ? AND provider_id = ?", blobID, providerID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrReplicaConflict
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	return replicaModelFromRow(row), nil
}

func (r *Repository) InsertBlob(ctx context.Context, blob *model.Blob) error {
	now := time.Now().UTC()
	return wrapPersistence(postgres.DB(ctx, r.db).Create(&blobRow{
		ID: blob.ID, DedupDomain: blob.DedupDomain, SHA256: blob.SHA256, Size: blob.Size,
		EncryptionKeyRef: blob.EncryptionKeyRef, RefCount: blob.RefCount, State: blob.State,
		PreferredProviderID: blob.PreferredProviderID,
		TombstonedAt:        blob.TombstonedAt, LegalHold: blob.LegalHold,
		CreatedAt: now,
	}).Error)
}

func (r *Repository) InsertReplica(ctx context.Context, replica *model.StorageReplica) error {
	now := time.Now().UTC()
	return wrapPersistence(postgres.DB(ctx, r.db).Create(&replicaRow{
		ID: replica.ID, BlobID: replica.BlobID, ProviderID: replica.ProviderID, ObjectKey: replica.ObjectKey,
		Checksum: replica.Checksum, State: replica.State, VerifiedAt: replica.VerifiedAt, DeleteAfter: replica.DeleteAfter,
		CreatedAt: now,
	}).Error)
}

func (r *Repository) SetPreferredProvider(ctx context.Context, blobID, providerID uuid.UUID) error {
	return wrapPersistence(postgres.DB(ctx, r.db).Model(&blobRow{}).Where("id = ?", blobID).
		Update("preferred_provider_id", providerID).Error)
}

func (r *Repository) UpdateReplica(ctx context.Context, replica *model.StorageReplica) error {
	updates := map[string]any{
		"object_key":   replica.ObjectKey,
		"checksum":     replica.Checksum,
		"state":        replica.State,
		"verified_at":  replica.VerifiedAt,
		"delete_after": replica.DeleteAfter,
	}
	return wrapPersistence(postgres.DB(ctx, r.db).Model(&replicaRow{}).Where("id = ?", replica.ID).Updates(updates).Error)
}

func (r *Repository) GetReplicaMigrationCheckpoint(ctx context.Context, blobID, targetProviderID uuid.UUID) (int64, error) {
	var row replicaMigrationRow
	err := postgres.DB(ctx, r.db).Where("blob_id = ? AND target_provider_id = ?", blobID, targetProviderID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, wrapPersistence(err)
	}
	return row.BytesCopied, nil
}

func (r *Repository) SaveReplicaMigrationCheckpoint(ctx context.Context, checkpoint repository.ReplicaMigrationCheckpoint) error {
	now := time.Now().UTC()
	row := replicaMigrationRow{
		BlobID: checkpoint.BlobID, TargetProviderID: checkpoint.TargetProviderID,
		TargetReplicaID: checkpoint.TargetReplicaID, SourceProviderID: checkpoint.SourceProviderID,
		BytesCopied: checkpoint.BytesCopied, TargetObjectKey: checkpoint.TargetObjectKey,
		PolicyVersion: checkpoint.PolicyVersion, UpdatedAt: now,
	}
	return wrapPersistence(postgres.DB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "blob_id"}, {Name: "target_provider_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"target_replica_id", "source_provider_id", "bytes_copied", "target_object_key", "policy_version", "updated_at"}),
	}).Create(&row).Error)
}

func (r *Repository) PromoteReplica(ctx context.Context, promotion repository.ReplicaPromotion) error {
	return wrapPersistence(postgres.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var target replicaRow
		if err := tx.Where("blob_id = ? AND provider_id = ?", promotion.BlobID, promotion.TargetProviderID).First(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrReplicaConflict
			}
			return err
		}
		if target.State != model.ReplicaStateAvailable {
			return domain.ErrReplicaConflict
		}
		deleteAfter := promotion.DeleteAfter
		result := tx.Model(&replicaRow{}).Where("blob_id = ? AND provider_id = ?", promotion.BlobID, promotion.SourceProviderID).
			Updates(map[string]any{"state": model.ReplicaStateDraining, "delete_after": deleteAfter})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrReplicaConflict
		}
		return tx.Model(&blobRow{}).Where("id = ?", promotion.BlobID).
			Update("preferred_provider_id", promotion.TargetProviderID).Error
	}))
}

func (r *Repository) ListResourceGrants(ctx context.Context, nodeIDs []uuid.UUID, principals []repository.PrincipalRef, permission model.ResourcePermission, at time.Time) ([]model.ResourceGrant, error) {
	if len(nodeIDs) == 0 || len(principals) == 0 {
		return []model.ResourceGrant{}, nil
	}
	var rows []resourceGrantRow
	query := postgres.DB(ctx, r.db).
		Where("node_id IN ? AND permission = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", nodeIDs, permission, at)
	conditions := make([]string, 0, len(principals))
	args := make([]any, 0, len(principals)*2)
	for _, principal := range principals {
		conditions = append(conditions, "(principal_type = ? AND principal_id = ?)")
		args = append(args, principal.Type, principal.ID)
	}
	err := query.Where(strings.Join(conditions, " OR "), args...).Find(&rows).Error
	if err != nil {
		return nil, wrapPersistence(err)
	}
	grants := make([]model.ResourceGrant, len(rows))
	for index, row := range rows {
		grants[index] = resourceGrantFromRow(row)
	}
	return grants, nil
}

func (r *Repository) CreateResourceGrant(ctx context.Context, grant model.ResourceGrant) error {
	if grant.PrincipalType == "" {
		grant.PrincipalType = model.PrincipalUser
	}
	if grant.PrincipalID == uuid.Nil {
		grant.PrincipalID = grant.GranteeID
	}
	if grant.GranteeID == uuid.Nil {
		grant.GranteeID = grant.PrincipalID
	}
	if grant.PrincipalID == uuid.Nil || grant.GranteeID != grant.PrincipalID {
		return domain.ErrResourceDenied
	}
	save := func(db *gorm.DB) error {
		now := time.Now().UTC()
		var existing []resourceGrantRow
		if err := db.Where("node_id = ? AND principal_type = ? AND principal_id = ? AND permission = ? AND effect = ? AND revoked_at IS NULL",
			grant.NodeID, grant.PrincipalType, grant.PrincipalID, grant.Permission, grant.Effect).Find(&existing).Error; err != nil {
			return err
		}
		for _, row := range existing {
			if row.ExpiresAt != nil && !row.ExpiresAt.After(now) {
				if err := db.Model(&resourceGrantRow{}).Where("id = ? AND revoked_at IS NULL", row.ID).Update("revoked_at", now).Error; err != nil {
					return err
				}
			}
		}
		return db.Create(&resourceGrantRow{
			ID: grant.ID, NodeID: grant.NodeID, OrganizationID: grant.OrganizationID, GranteeID: grant.GranteeID,
			PrincipalType: grant.PrincipalType, PrincipalID: grant.PrincipalID, Permission: grant.Permission,
			Effect: grant.Effect, InheritToChildren: grant.InheritToChildren, ExpiresAt: grant.ExpiresAt,
			RevokedAt: grant.RevokedAt, PolicyVersion: grant.PolicyVersion, CreatedAt: now,
		}).Error
	}
	if _, err := postgres.RequireTx(ctx); err == nil {
		return wrapPersistence(save(postgres.DB(ctx, r.db)))
	}
	return wrapPersistence(postgres.DB(ctx, r.db).Transaction(save))
}

func (r *Repository) RevokeResourceGrant(ctx context.Context, nodeID, grantID uuid.UUID) error {
	now := time.Now().UTC()
	result := postgres.DB(ctx, r.db).Model(&resourceGrantRow{}).
		Where("id = ? AND node_id = ? AND revoked_at IS NULL", grantID, nodeID).
		Update("revoked_at", now)
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
	now := time.Now().UTC()
	if err := postgres.DB(ctx, r.db).Where("node_id = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", nodeID, now).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, wrapPersistence(err)
	}
	result := make([]model.ResourceGrant, len(rows))
	for i, row := range rows {
		result[i] = resourceGrantFromRow(row)
	}
	return result, nil
}

func resourceGrantFromRow(row resourceGrantRow) model.ResourceGrant {
	return model.ResourceGrant{
		ID: row.ID, NodeID: row.NodeID, OrganizationID: row.OrganizationID, GranteeID: row.GranteeID,
		PrincipalType: row.PrincipalType, PrincipalID: row.PrincipalID, Permission: row.Permission,
		Effect: row.Effect, InheritToChildren: row.InheritToChildren, ExpiresAt: row.ExpiresAt,
		RevokedAt: row.RevokedAt, PolicyVersion: row.PolicyVersion,
	}
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
		case "storage_providers_pkey", "storage_providers_organization_id_name_key":
			return domain.ErrStorageProviderConflict
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
	case strings.Contains(message, "storage_providers.id"),
		strings.Contains(message, "storage_providers.organization_id, storage_providers.name"):
		return domain.ErrStorageProviderConflict
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
	if isSQLiteBusy(err) {
		return domain.ErrTransactionRetry
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
		domain.ErrResourceDenied, domain.ErrCrossOrganizationGrant, domain.ErrAnonymousLinkDisabled,
		domain.ErrCapabilityScopeEmpty, domain.ErrCapabilityInvalid, domain.ErrCapabilityNotFound,
		domain.ErrCapabilityRevoked, domain.ErrCapabilityExpired, domain.ErrExportNotFound,
		domain.ErrControlledExportScope,
		domain.ErrGrantNotFound, domain.ErrNodeStateConflict,
		domain.ErrTransferNotFound, domain.ErrTransferConflict, domain.ErrTransferIdempotencyConflict,
		domain.ErrTransferVersionConflict, domain.ErrTransferStatusConflict,
		domain.ErrTransferPartConflict, domain.ErrTransferPartOverlap,
		domain.ErrStorageProviderNotFound, domain.ErrStorageProviderConflict,
		model.ErrInvalidStorageProvider,
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
		equalTimePointers(row.VerifiedAt, replica.VerifiedAt) &&
		equalTimePointers(row.DeleteAfter, replica.DeleteAfter)
}

func replicaModelFromRow(row replicaRow) *model.StorageReplica {
	return &model.StorageReplica{
		ID: row.ID, BlobID: row.BlobID, ProviderID: row.ProviderID, ObjectKey: row.ObjectKey,
		Checksum: row.Checksum, State: row.State, VerifiedAt: row.VerifiedAt, DeleteAfter: row.DeleteAfter,
	}
}

func equalTimePointers(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Truncate(time.Microsecond).Equal(right.Truncate(time.Microsecond))
}

func (r *Repository) CreateTransfer(ctx context.Context, session *model.TransferSession) error {
	now := time.Now().UTC()
	row := transferRowFromModel(session, now)
	db := postgres.DB(ctx, r.db)
	result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		if db.Dialector.Name() == "sqlite" && isSQLiteBusy(result.Error) {
			return domain.ErrTransactionRetry
		}
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrTransferConflict
	}
	return nil
}

func (r *Repository) GetTransferByID(ctx context.Context, id uuid.UUID) (*model.TransferSession, error) {
	var row transferSessionRow
	err := postgres.DB(ctx, r.db).Where("id = ?", id).First(&row).Error
	return transferResult(row, err)
}

func (r *Repository) GetTransferByIdempotency(ctx context.Context, workspaceID uuid.UUID, key string) (*model.TransferSession, error) {
	var row transferSessionRow
	err := postgres.DB(ctx, r.db).Where("workspace_id = ? AND idempotency_key = ?", workspaceID, key).First(&row).Error
	return transferResult(row, err)
}

type transferCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

func (r *Repository) ListTransfers(
	ctx context.Context,
	workspaceID uuid.UUID,
	status *model.TransferStatus,
	limit int,
	cursor string,
) ([]*model.TransferSession, string, error) {
	if workspaceID == uuid.Nil {
		return nil, "", model.ErrInvalidTransfer
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	query := postgres.DB(ctx, r.db).Where("workspace_id = ?", workspaceID)
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if cursor != "" {
		boundary, err := decodeTransferCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		query = query.Where(
			"(created_at < ?) OR (created_at = ? AND id < ?)",
			boundary.CreatedAt, boundary.CreatedAt, boundary.ID,
		)
	}
	var rows []transferSessionRow
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", wrapPersistence(err)
	}
	next := ""
	if len(rows) > limit {
		var err error
		next, err = encodeTransferCursor(rows[limit-1])
		if err != nil {
			return nil, "", err
		}
		rows = rows[:limit]
	}
	out := make([]*model.TransferSession, 0, len(rows))
	for _, row := range rows {
		session, err := transferResult(row, nil)
		if err != nil {
			return nil, "", err
		}
		out = append(out, session)
	}
	return out, next, nil
}

func encodeTransferCursor(row transferSessionRow) (string, error) {
	payload, err := json.Marshal(transferCursor{
		CreatedAt: row.CreatedAt.UTC(),
		ID:        row.ID,
	})
	if err != nil {
		return "", domain.ErrPersistence
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeTransferCursor(value string) (transferCursor, error) {
	var cursor transferCursor
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(payload, &cursor) != nil ||
		cursor.CreatedAt.IsZero() || cursor.ID == uuid.Nil {
		return transferCursor{}, model.ErrInvalidTransfer
	}
	cursor.CreatedAt = cursor.CreatedAt.UTC()
	return cursor, nil
}

func (r *Repository) LockTransfer(ctx context.Context, id uuid.UUID) (*model.TransferSession, error) {
	tx, err := postgres.RequireTx(ctx)
	if err != nil {
		return nil, domain.ErrPersistence
	}
	if tx.Dialector.Name() == "sqlite" {
		result := tx.Exec("UPDATE filesystem.transfer_sessions SET updated_at = updated_at WHERE id = ?", id)
		if isSQLiteBusy(result.Error) {
			return nil, domain.ErrTransactionRetry
		}
		if result.Error != nil {
			return nil, wrapPersistence(result.Error)
		}
		if result.RowsAffected == 0 {
			return nil, domain.ErrTransferNotFound
		}
	}
	var row transferSessionRow
	query := tx.Where("id = ?", id)
	if tx.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err = query.First(&row).Error
	return transferResult(row, err)
}

func (r *Repository) ListTransferParts(ctx context.Context, sessionID uuid.UUID) ([]model.TransferPart, error) {
	db := postgres.DB(ctx, r.db)
	var session transferSessionRow
	if err := db.Select("id", "expected_size").Where("id = ?", sessionID).First(&session).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTransferNotFound
	} else if err != nil {
		return nil, wrapPersistence(err)
	}
	var rows []transferPartRow
	if err := db.Where("session_id = ?", sessionID).Order("part_number ASC").Find(&rows).Error; err != nil {
		return nil, wrapPersistence(err)
	}
	parts := make([]model.TransferPart, 0, len(rows))
	for _, row := range rows {
		part, err := model.NewTransferPart(model.TransferPartInput{
			ID: row.ID, SessionID: row.SessionID, PartNumber: row.PartNumber,
			Offset: row.Offset, Size: row.Size, SessionSize: session.ExpectedSize,
			Checksum: stringValueOrEmpty(row.Checksum), ETag: row.ETag,
		})
		if err != nil {
			return nil, domain.ErrPersistence
		}
		parts = append(parts, *part)
	}
	return parts, nil
}

func (r *Repository) UpsertTransferPart(ctx context.Context, part *model.TransferPart) error {
	save := func(txCtx context.Context) error {
		session, err := r.LockTransfer(txCtx, part.SessionID())
		if err != nil {
			return err
		}
		if part.Offset() > session.ExpectedSize() || part.Size() > session.ExpectedSize()-part.Offset() {
			return model.ErrInvalidTransferPart
		}
		db := postgres.DB(txCtx, r.db)
		var existing transferPartRow
		err = db.Where("session_id = ? AND part_number = ?", part.SessionID(), part.PartNumber()).First(&existing).Error
		if err == nil {
			if existing.Offset == part.Offset() && existing.Size == part.Size() &&
				stringValueOrEmpty(existing.Checksum) == part.Checksum() && existing.ETag == part.ETag() {
				return nil
			}
			return domain.ErrTransferPartConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return wrapPersistence(err)
		}
		var overlap int64
		if err := db.Model(&transferPartRow{}).
			Where("session_id = ? AND offset_bytes < ? AND offset_bytes + size_bytes > ?",
				part.SessionID(), part.Offset()+part.Size(), part.Offset()).
			Count(&overlap).Error; err != nil {
			return wrapPersistence(err)
		}
		if overlap > 0 {
			return domain.ErrTransferPartOverlap
		}
		now := time.Now().UTC()
		row := transferPartRow{
			ID: part.ID(), SessionID: part.SessionID(), PartNumber: part.PartNumber(),
			Offset: part.Offset(), Size: part.Size(), ETag: part.ETag(),
			CreatedAt: now, UpdatedAt: now,
		}
		if checksum := part.Checksum(); checksum != "" {
			row.Checksum = &checksum
		}
		err = db.Create(&row).Error
		if isSQLiteBusy(err) {
			return domain.ErrTransactionRetry
		}
		var pgErr *pgconn.PgError
		if errors.Is(err, gorm.ErrDuplicatedKey) ||
			(errors.As(err, &pgErr) && pgErr.Code == "23505") ||
			strings.Contains(strings.ToLower(fmt.Sprint(err)), "unique constraint failed") {
			return domain.ErrTransferPartConflict
		}
		return wrapPersistence(err)
	}
	if _, err := postgres.RequireTx(ctx); err == nil {
		return save(ctx)
	}
	return postgres.NewTransactor(r.db).WithinTransaction(ctx, save)
}

func (r *Repository) SaveTransferStateCAS(ctx context.Context, workspaceID, id uuid.UUID, expectedVersion int64, patch repository.TransferStatePatch) error {
	updates := map[string]any{
		"status":     patch.StatusTo,
		"version":    gorm.Expr("version + 1"),
		"updated_at": time.Now().UTC(),
	}
	if patch.ActualSize != nil {
		updates["actual_size"] = *patch.ActualSize
	}
	if patch.ActualSHA256 != "" {
		updates["actual_sha256"] = patch.ActualSHA256
	}
	if patch.ProviderID != nil {
		updates["provider_id"] = *patch.ProviderID
	}
	if patch.ScanResult != nil {
		updates["scan_clean"] = patch.ScanResult.Clean
		updates["scan_threat_name"] = patch.ScanResult.ThreatName
		updates["scan_engine"] = patch.ScanResult.Engine
		updates["scan_signature"] = patch.ScanResult.Signature
		updates["scan_completed_at"] = patch.ScanResult.CompletedAt
	}
	if patch.ErrorCode != nil {
		updates["error_code"] = *patch.ErrorCode
	}
	db := postgres.DB(ctx, r.db)
	result := db.Model(&transferSessionRow{}).
		Where("workspace_id = ? AND id = ? AND version = ? AND status = ?", workspaceID, id, expectedVersion, patch.StatusFrom).
		Updates(updates)
	if result.Error != nil {
		if db.Dialector.Name() == "sqlite" && isSQLiteBusy(result.Error) {
			return domain.ErrTransactionRetry
		}
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 1 {
		return nil
	}
	var row transferSessionRow
	err := db.Select("id", "version", "status").Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrTransferNotFound
	}
	if err != nil {
		return wrapPersistence(err)
	}
	if row.Version != expectedVersion {
		return domain.ErrTransferVersionConflict
	}
	return domain.ErrTransferStatusConflict
}

func transferRowFromModel(session *model.TransferSession, now time.Time) transferSessionRow {
	row := transferSessionRow{
		ID: session.ID(), WorkspaceID: session.WorkspaceID(), NamespaceID: session.NamespaceID(),
		TargetParentID: session.TargetParentID(), TargetName: session.TargetName(), Protocol: session.Protocol(),
		ExpectedSize: session.ExpectedSize(),
		ProviderID:   session.ProviderID(), TemporaryObjectKey: session.TemporaryObjectKey(),
		Status: session.Status(), Classification: session.Classification(), ErrorCode: session.ErrorCode(),
		IdempotencyKey: session.IdempotencyKey(), RequestHash: session.RequestHash(),
		ExpiresAt: model.CanonicalTransferExpiry(session.ExpiresAt()), Version: session.Version(),
		CreatedBy: session.CreatedBy(), CreatedAt: now, UpdatedAt: now,
	}
	if size, ok := session.ActualSize(); ok {
		row.ActualSize = &size
	}
	if checksum := session.ExpectedSHA256(); checksum != "" {
		row.ExpectedSHA256 = &checksum
	}
	if checksum := session.ActualSHA256(); checksum != "" {
		row.ActualSHA256 = &checksum
	}
	if scan := session.ScanResult(); scan != nil {
		row.ScanClean = &scan.Clean
		row.ScanThreatName, row.ScanEngine, row.ScanSignature = scan.ThreatName, scan.Engine, scan.Signature
		row.ScanCompletedAt = &scan.CompletedAt
	}
	return row
}

func transferResult(row transferSessionRow, err error) (*model.TransferSession, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrTransferNotFound
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	var scan *model.TransferScanResult
	if row.ScanClean != nil {
		completed := time.Time{}
		if row.ScanCompletedAt != nil {
			completed = *row.ScanCompletedAt
		}
		scan = &model.TransferScanResult{
			Clean: *row.ScanClean, ThreatName: row.ScanThreatName, Engine: row.ScanEngine,
			Signature: row.ScanSignature, CompletedAt: completed,
		}
	}
	return model.RestoreTransferSession(model.PersistedTransferSession{
		Input: model.TransferSessionInput{
			ID: row.ID, WorkspaceID: row.WorkspaceID, NamespaceID: row.NamespaceID,
			TargetParentID: row.TargetParentID, TargetName: row.TargetName, CreatedBy: row.CreatedBy,
			Protocol: row.Protocol, ExpectedSize: row.ExpectedSize, ExpectedSHA256: stringValueOrEmpty(row.ExpectedSHA256),
			IdempotencyKey: row.IdempotencyKey, RequestHash: row.RequestHash, TemporaryObjectKey: row.TemporaryObjectKey,
			ExpiresAt: row.ExpiresAt, ProviderID: row.ProviderID, Classification: row.Classification,
		},
		Status: row.Status, ActualSize: row.ActualSize, ActualSHA256: stringValueOrEmpty(row.ActualSHA256),
		ScanResult: scan, ErrorCode: row.ErrorCode, Version: row.Version,
	})
}

func stringValueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (r *Repository) CreateStorageProvider(ctx context.Context, provider *model.StorageProvider) error {
	now := time.Now().UTC()
	row := storageProviderRowFromModel(provider, now)
	if err := postgres.DB(ctx, r.db).Create(&row).Error; err != nil {
		return wrapPersistence(err)
	}
	return nil
}

func (r *Repository) GetStorageProvider(ctx context.Context, id uuid.UUID) (*model.StorageProvider, error) {
	var row storageProviderRow
	err := postgres.DB(ctx, r.db).Where("id = ?", id).First(&row).Error
	return storageProviderResult(row, err)
}

func (r *Repository) CreateLifecycleJob(context.Context, *model.LifecycleJob) error {
	return domain.ErrPersistence
}

func (r *Repository) ConfirmBlobGCEligible(ctx context.Context, blobID uuid.UUID, grace time.Duration, now time.Time) (repository.BlobGCContext, error) {
	var snapshot repository.BlobGCContext
	err := postgres.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var row blobRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", blobID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrBlobConflict
			}
			return err
		}
		var liveRevisionCount int64
		if err := tx.Table(revisionRow{}.TableName()).
			Joins("INNER JOIN "+nodeRow{}.TableName()+" n ON n.id = "+revisionRow{}.TableName()+".node_id").
			Where(revisionRow{}.TableName()+".blob_id = ? AND n.state <> ?", blobID, model.NodeStatePurged).
			Count(&liveRevisionCount).Error; err != nil {
			return err
		}
		var activeLinkCount int64
		if err := tx.Table(fileLinkRow{}.TableName()).
			Joins("INNER JOIN "+revisionRow{}.TableName()+" fr ON fr.id = "+fileLinkRow{}.TableName()+".revision_id").
			Where("fr.blob_id = ?", blobID).
			Count(&activeLinkCount).Error; err != nil {
			return err
		}
		var retentionHoldCount int64
		if err := tx.Table(revisionRow{}.TableName()).
			Joins("INNER JOIN "+nodeRow{}.TableName()+" n ON n.id = "+revisionRow{}.TableName()+".node_id").
			Where(revisionRow{}.TableName()+".blob_id = ? AND n.state = ?", blobID, model.NodeStateRetentionHold).
			Count(&retentionHoldCount).Error; err != nil {
			return err
		}
		tombstonedAt := time.Time{}
		if row.TombstonedAt != nil {
			tombstonedAt = row.TombstonedAt.UTC()
		}
		snapshot = repository.BlobGCContext{
			BlobID: blobID, State: row.State, RefCount: row.RefCount,
			TombstonedAt: tombstonedAt, LegalHold: row.LegalHold || retentionHoldCount > 0,
			LiveRevisionCount: liveRevisionCount, ActiveLinkCount: activeLinkCount,
		}
		return repository.ValidateBlobGCEligible(snapshot, grace, now)
	})
	return snapshot, wrapPersistence(err)
}

func (r *Repository) ListReplicasForGC(ctx context.Context, blobID uuid.UUID) ([]model.StorageReplica, error) {
	var rows []replicaRow
	err := postgres.DB(ctx, r.db).
		Where("blob_id = ? AND state <> ?", blobID, model.ReplicaStateDeleted).
		Order("id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, wrapPersistence(err)
	}
	out := make([]model.StorageReplica, len(rows))
	for i, row := range rows {
		out[i] = *replicaModelFromRow(row)
	}
	return out, nil
}

func (r *Repository) FinalizeBlobGC(ctx context.Context, blobID uuid.UUID) error {
	now := time.Now().UTC()
	return wrapPersistence(postgres.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&blobRow{}).Where("id = ?", blobID).Updates(map[string]any{
			"state": model.BlobStateDeleted,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrBlobConflict
		}
		return tx.Model(&replicaRow{}).Where("blob_id = ? AND state <> ?", blobID, model.ReplicaStateDeleted).
			Updates(map[string]any{"state": model.ReplicaStateDeleted, "verified_at": now}).Error
	}))
}

func (r *Repository) MarkBlobGCPending(ctx context.Context, blobID uuid.UUID) error {
	result := postgres.DB(ctx, r.db).Model(&blobRow{}).Where("id = ?", blobID).
		Update("state", model.BlobStateGCPending)
	if result.Error != nil {
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrBlobConflict
	}
	return nil
}

func (r *Repository) SaveLifecycleJobCheckpoint(context.Context, uuid.UUID, map[string]any) error {
	return domain.ErrPersistence
}

func (r *Repository) GetNodeByID(ctx context.Context, nodeID uuid.UUID) (*model.Node, error) {
	var row nodeRow
	err := postgres.DB(ctx, r.db).Where("id = ?", nodeID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNodeNotFound
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	return nodeFromRow(row), nil
}

func (r *Repository) CreateArtifact(ctx context.Context, artifact model.Artifact, link model.FileLink) error {
	refs, err := json.Marshal(artifact.InputRefs)
	if err != nil {
		return wrapPersistence(err)
	}
	row := artifactRow{
		ID: artifact.ID, WorkspaceID: artifact.WorkspaceID, NamespaceID: artifact.NamespaceID,
		ArtifactType: artifact.ArtifactType, ProducerType: artifact.ProducerType, ProducerID: artifact.ProducerID,
		RunID: artifact.RunID, InputRefs: string(refs), FileID: artifact.FileID, RevisionID: artifact.RevisionID,
		RetentionPolicyID: artifact.RetentionPolicyID, CreatedAt: artifact.CreatedAt,
	}
	linkRow := fileLinkRow{
		ID: link.ID, NodeID: link.NodeID, RevisionID: link.RevisionID, LinkType: link.LinkType, LinkID: link.LinkID, CreatedAt: link.CreatedAt,
	}
	db := postgres.DB(ctx, r.db)
	if err := db.Create(&row).Error; err != nil {
		return wrapPersistence(err)
	}
	return wrapPersistence(db.Create(&linkRow).Error)
}

func (r *Repository) GetArtifact(ctx context.Context, id uuid.UUID) (model.Artifact, error) {
	var row artifactRow
	err := postgres.DB(ctx, r.db).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Artifact{}, domain.ErrOutcomeUnknown
	}
	if err != nil {
		return model.Artifact{}, wrapPersistence(err)
	}
	return artifactFromRow(row), nil
}

func (r *Repository) ListArtifacts(ctx context.Context, workspaceID uuid.UUID, runID *uuid.UUID, limit int, cursor string) ([]model.Artifact, string, error) {
	if limit <= 0 {
		limit = 50
	}
	query := postgres.DB(ctx, r.db).Where("workspace_id = ?", workspaceID)
	if runID != nil {
		query = query.Where("run_id = ?", *runID)
	}
	if cursor != "" {
		query = query.Where("id > ?", cursor)
	}
	var rows []artifactRow
	if err := query.Order("id ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", wrapPersistence(err)
	}
	next := ""
	if len(rows) > limit {
		next = rows[limit].ID.String()
		rows = rows[:limit]
	}
	out := make([]model.Artifact, 0, len(rows))
	for _, row := range rows {
		out = append(out, artifactFromRow(row))
	}
	return out, next, nil
}

func artifactFromRow(row artifactRow) model.Artifact {
	refs := []uuid.UUID{}
	_ = json.Unmarshal([]byte(row.InputRefs), &refs)
	return model.Artifact{
		ID: row.ID, WorkspaceID: row.WorkspaceID, NamespaceID: row.NamespaceID,
		ArtifactType: row.ArtifactType, ProducerType: row.ProducerType, ProducerID: row.ProducerID,
		RunID: row.RunID, InputRefs: refs, FileID: row.FileID, RevisionID: row.RevisionID,
		RetentionPolicyID: row.RetentionPolicyID, CreatedAt: row.CreatedAt,
	}
}

func (r *Repository) SearchFileNodes(ctx context.Context, filter repository.FileSearchFilter) ([]uuid.UUID, string, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	query := postgres.DB(ctx, r.db).Model(&nodeRow{}).
		Where("namespace_id = ?", filter.NamespaceID).
		Where("state = ?", model.NodeStateActive)
	if filter.OwnerID != nil {
		query = query.Where("owner_id = ?", *filter.OwnerID)
	}
	needle := strings.ToLower(strings.TrimSpace(norm.NFC.String(filter.Query)))
	if needle != "" {
		query = query.Where("normalized_name LIKE ?", "%"+needle+"%")
	}
	if cursor, err := uuid.Parse(strings.TrimSpace(filter.Cursor)); err == nil && cursor != uuid.Nil {
		query = query.Where("id > ?", cursor)
	}
	var rows []nodeRow
	if err := query.Order("id ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", wrapPersistence(err)
	}
	next := ""
	if len(rows) > limit {
		next = rows[limit].ID.String()
		rows = rows[:limit]
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, next, nil
}

func (r *Repository) CreateFileLink(ctx context.Context, link model.FileLink) error {
	row := fileLinkRow{
		ID: link.ID, NodeID: link.NodeID, RevisionID: link.RevisionID,
		LinkType: link.LinkType, LinkID: link.LinkID, CreatedAt: link.CreatedAt,
	}
	return wrapPersistence(postgres.DB(ctx, r.db).Create(&row).Error)
}

func (r *Repository) CreateNamespaceRootLink(ctx context.Context, link model.FileLink) error {
	if link.LinkType != model.FileLinkTypeNamespaceRoot ||
		link.ID == uuid.Nil || link.LinkID == uuid.Nil || link.NodeID == uuid.Nil {
		return domain.ErrPersistence
	}
	row := fileLinkRow{
		ID: link.ID, NodeID: link.NodeID, RevisionID: link.RevisionID,
		LinkType: link.LinkType, LinkID: link.LinkID, CreatedAt: link.CreatedAt,
	}
	return wrapPersistence(postgres.DB(ctx, r.db).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error)
}

func (r *Repository) GetFileLink(ctx context.Context, nodeID uuid.UUID, revisionID *uuid.UUID, linkType string) (*model.FileLink, error) {
	query := postgres.DB(ctx, r.db).Where("node_id = ? AND link_type = ?", nodeID, linkType)
	if revisionID != nil {
		query = query.Where("revision_id = ?", *revisionID)
	}
	var row fileLinkRow
	err := query.Order("created_at DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNodeNotFound
	}
	if err != nil {
		return nil, wrapPersistence(err)
	}
	return &model.FileLink{
		ID: row.ID, NodeID: row.NodeID, RevisionID: row.RevisionID,
		LinkType: row.LinkType, LinkID: row.LinkID, CreatedAt: row.CreatedAt,
	}, nil
}

var _ repository.FilesystemRepository = (*Repository)(nil)

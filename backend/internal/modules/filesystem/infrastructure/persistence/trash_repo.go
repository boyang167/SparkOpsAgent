package persistence

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func (r *Repository) ListTrashEntries(ctx context.Context, query repository.TrashListQuery) ([]repository.TrashEntry, string, error) {
	if query.NamespaceID == uuid.Nil {
		return nil, "", domain.ErrPersistence
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	cursorDeletedAt, cursorID, err := parseTrashCursor(query.Cursor)
	if err != nil {
		return nil, "", err
	}
	ns, err := r.GetNamespace(ctx, query.NamespaceID)
	if err != nil {
		return nil, "", err
	}
	type trashRow struct {
		nodeRow
	}
	cursorClause := ""
	args := []any{query.NamespaceID, query.NamespaceID, query.NamespaceID}
	if !cursorDeletedAt.IsZero() && cursorID != uuid.Nil {
		cursorClause = " AND (n.deleted_at < ? OR (n.deleted_at = ? AND n.id < ?))"
		args = append(args, cursorDeletedAt, cursorDeletedAt, cursorID)
	}
	args = append(args, limit+1)
	sql := `
WITH RECURSIVE upward AS (
	SELECT id, parent_id, state, id AS entry_id
	FROM filesystem.nodes
	WHERE namespace_id = ? AND state = 'TRASHED'
	UNION ALL
	SELECT p.id, p.parent_id, p.state, u.entry_id
	FROM upward u
	JOIN filesystem.nodes p ON p.id = u.parent_id AND p.namespace_id = ?
	WHERE u.parent_id IS NOT NULL
)
SELECT n.*
FROM filesystem.nodes n
WHERE n.namespace_id = ?
  AND n.state = 'TRASHED'
  AND NOT EXISTS (
	SELECT 1 FROM upward u
	WHERE u.entry_id = n.id AND u.id <> n.id AND u.state = 'TRASHED'
  )` + cursorClause + `
ORDER BY n.deleted_at DESC, n.id DESC
LIMIT ?`
	var rows []nodeRow
	if err := postgres.DB(ctx, r.db).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, "", wrapPersistence(err)
	}
	next := ""
	if len(rows) > limit {
		last := rows[limit-1]
		next = encodeTrashCursor(deletedAtValue(last.DeletedAt), last.ID)
		rows = rows[:limit]
	}
	out := make([]repository.TrashEntry, 0, len(rows))
	nodeByID := make(map[uuid.UUID]nodeRow, len(rows))
	for _, row := range rows {
		nodeByID[row.ID] = row
	}
	for _, row := range rows {
		path, pathErr := r.buildNodeOriginalPath(ctx, ns, row.ID, nodeByID)
		if pathErr != nil {
			return nil, "", pathErr
		}
		deletedAt := deletedAtValue(row.DeletedAt)
		if deletedAt.IsZero() {
			deletedAt = row.UpdatedAt.UTC()
		}
		deletedBy := uuid.Nil
		if row.DeletedBy != nil {
			deletedBy = *row.DeletedBy
		}
		out = append(out, repository.TrashEntry{
			Node: *nodeFromRow(row), OriginalPath: path,
			DeletedAt: deletedAt, DeletedBy: deletedBy,
		})
	}
	return out, next, nil
}

func (r *Repository) RestoreNode(ctx context.Context, namespaceID, nodeID uuid.UUID) error {
	node, err := r.GetNodeIncludingTrashed(ctx, namespaceID, nodeID)
	if err != nil {
		return err
	}
	if node.State != model.NodeStateTrashed {
		return domain.ErrNodeStateConflict
	}
	if node.ParentID != nil {
		parent, parentErr := r.GetNodeIncludingTrashed(ctx, namespaceID, *node.ParentID)
		if parentErr != nil {
			if errors.Is(parentErr, domain.ErrNodeNotFound) {
				return domain.ErrRestoreParentUnavailable
			}
			return parentErr
		}
		if parent.State != model.NodeStateActive || parent.Type != model.NodeTypeDirectory {
			return domain.ErrRestoreParentUnavailable
		}
	}
	conflictQuery := postgres.DB(ctx, r.db).Model(&nodeRow{}).
		Where("namespace_id = ? AND normalized_name = ? AND state = ?", namespaceID, node.NormalizedName, model.NodeStateActive)
	if node.ParentID == nil {
		conflictQuery = conflictQuery.Where("parent_id IS NULL")
	} else {
		conflictQuery = conflictQuery.Where("parent_id = ?", *node.ParentID)
	}
	var conflictCount int64
	if err := conflictQuery.Count(&conflictCount).Error; err != nil {
		return wrapPersistence(err)
	}
	if conflictCount > 0 {
		return domain.ErrRestoreNameConflict
	}
	now := time.Now().UTC()
	result := postgres.DB(ctx, r.db).Model(&nodeRow{}).
		Where("namespace_id = ? AND id = ? AND state = ?", namespaceID, nodeID, model.NodeStateTrashed).
		Updates(map[string]any{
			"state":      model.NodeStateActive,
			"deleted_at": nil,
			"deleted_by": nil,
			"updated_at": now,
		})
	if result.Error != nil {
		if conflict := mapUniqueConflict(result.Error); conflict != nil {
			return conflict
		}
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNodeStateConflict
	}
	return nil
}

func (r *Repository) PurgeTrashedSubtree(ctx context.Context, namespaceID, entryID uuid.UUID, now time.Time) (repository.PurgeSummary, error) {
	now = now.UTC()
	if _, err := postgres.RequireTx(ctx); err != nil {
		return repository.PurgeSummary{}, domain.ErrPersistence
	}
	var entry nodeRow
	db := postgres.DB(ctx, r.db)
	if db.Dialector.Name() == "postgres" {
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("namespace_id = ? AND id = ?", namespaceID, entryID).First(&entry).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return repository.PurgeSummary{}, domain.ErrNodeNotFound
			}
			return repository.PurgeSummary{}, wrapPersistence(err)
		}
	} else {
		if err := db.Where("namespace_id = ? AND id = ?", namespaceID, entryID).First(&entry).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return repository.PurgeSummary{}, domain.ErrNodeNotFound
			}
			return repository.PurgeSummary{}, wrapPersistence(err)
		}
	}
	if entry.State != model.NodeStateTrashed {
		return repository.PurgeSummary{}, domain.ErrNodeStateConflict
	}
	var subtree []nodeRow
	if err := db.Raw(`
WITH RECURSIVE subtree AS (
	SELECT * FROM filesystem.nodes WHERE namespace_id = ? AND id = ?
	UNION ALL
	SELECT child.* FROM filesystem.nodes child
	JOIN subtree parent ON child.parent_id = parent.id
	WHERE child.namespace_id = ?
)
SELECT * FROM subtree`, namespaceID, entryID, namespaceID).Scan(&subtree).Error; err != nil {
		return repository.PurgeSummary{}, wrapPersistence(err)
	}
	if len(subtree) == 0 {
		return repository.PurgeSummary{}, domain.ErrNodeNotFound
	}
	nodeIDs := make([]uuid.UUID, 0, len(subtree))
	var fileCount int64
	for _, row := range subtree {
		if row.State == model.NodeStateRetentionHold {
			return repository.PurgeSummary{}, domain.ErrRetentionHold
		}
		nodeIDs = append(nodeIDs, row.ID)
		if row.Type == model.NodeTypeFile {
			fileCount++
		}
	}
	var revisions []revisionRow
	if err := db.Where("node_id IN ?", nodeIDs).Find(&revisions).Error; err != nil {
		return repository.PurgeSummary{}, wrapPersistence(err)
	}
	var byteCount int64
	blobIDs := make([]uuid.UUID, 0, len(revisions))
	seenBlobs := make(map[uuid.UUID]struct{})
	for _, revision := range revisions {
		byteCount += revision.Size
		if _, seen := seenBlobs[revision.BlobID]; !seen {
			seenBlobs[revision.BlobID] = struct{}{}
			var blob blobRow
			if err := db.Where("id = ?", revision.BlobID).First(&blob).Error; err != nil {
				return repository.PurgeSummary{}, wrapPersistence(err)
			}
			if blob.LegalHold {
				return repository.PurgeSummary{}, domain.ErrRetentionHold
			}
			blobIDs = append(blobIDs, revision.BlobID)
		}
	}
	pending := db.Model(&nodeRow{}).Where("namespace_id = ? AND id IN ? AND state <> ?", namespaceID, nodeIDs, model.NodeStatePurged).
		Updates(map[string]any{"state": model.NodeStatePurgePending, "updated_at": now})
	if pending.Error != nil {
		return repository.PurgeSummary{}, wrapPersistence(pending.Error)
	}
	if err := r.ReleaseNamespaceQuotaUsage(ctx, namespaceID, byteCount, fileCount); err != nil {
		return repository.PurgeSummary{}, err
	}
	for _, revision := range revisions {
		if err := r.DecrementBlobRef(ctx, revision.BlobID, now); err != nil {
			return repository.PurgeSummary{}, err
		}
	}
	purged := db.Model(&nodeRow{}).Where("namespace_id = ? AND id IN ?", namespaceID, nodeIDs).
		Updates(map[string]any{"state": model.NodeStatePurged, "updated_at": now})
	if purged.Error != nil {
		return repository.PurgeSummary{}, wrapPersistence(purged.Error)
	}
	return repository.PurgeSummary{NodeIDs: nodeIDs, FileCount: fileCount, ByteCount: byteCount, BlobIDs: blobIDs}, nil
}

func (r *Repository) ReleaseNamespaceQuotaUsage(ctx context.Context, namespaceID uuid.UUID, bytes, files int64) error {
	if namespaceID == uuid.Nil || bytes < 0 || files < 0 {
		return domain.ErrPersistence
	}
	if bytes == 0 && files == 0 {
		return nil
	}
	db := postgres.DB(ctx, r.db)
	result := db.Model(&quotaAccountRow{}).
		Where("namespace_id = ? AND used_bytes >= ? AND used_files >= ?", namespaceID, bytes, files).
		Updates(map[string]any{
			"used_bytes": gorm.Expr("used_bytes - ?", bytes),
			"used_files": gorm.Expr("used_files - ?", files),
			"version":    gorm.Expr("version + 1"),
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrPersistence
	}
	return nil
}

func (r *Repository) DecrementBlobRef(ctx context.Context, blobID uuid.UUID, now time.Time) error {
	now = now.UTC()
	db := postgres.DB(ctx, r.db)
	var row blobRow
	lock := db
	if db.Dialector.Name() == "postgres" {
		lock = db.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := lock.Where("id = ?", blobID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrBlobConflict
		}
		return wrapPersistence(err)
	}
	if row.RefCount <= 0 {
		return domain.ErrBlobConflict
	}
	updates := map[string]any{"ref_count": row.RefCount - 1}
	if row.RefCount == 1 {
		updates["state"] = model.BlobStateTombstoned
		updates["tombstoned_at"] = now
	}
	result := db.Model(&blobRow{}).Where("id = ? AND ref_count = ?", blobID, row.RefCount).Updates(updates)
	if result.Error != nil {
		return wrapPersistence(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrBlobConflict
	}
	return nil
}

func (r *Repository) buildNodeOriginalPath(ctx context.Context, ns *model.Namespace, nodeID uuid.UUID, warm map[uuid.UUID]nodeRow) (string, error) {
	parts := make([]string, 0, 8)
	visited := make(map[uuid.UUID]struct{})
	currentID := nodeID
	for {
		if _, loop := visited[currentID]; loop {
			break
		}
		visited[currentID] = struct{}{}
		row, found := warm[currentID]
		if !found {
			var loaded nodeRow
			err := postgres.DB(ctx, r.db).Where("namespace_id = ? AND id = ?", ns.ID, currentID).First(&loaded).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}
			if err != nil {
				return "", wrapPersistence(err)
			}
			row = loaded
		}
		if ns.RootNodeID == nil || row.ID != *ns.RootNodeID {
			parts = append(parts, row.Name)
		}
		if row.ParentID == nil {
			break
		}
		currentID = *row.ParentID
	}
	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	if len(parts) == 0 {
		return "/", nil
	}
	return "/" + strings.Join(parts, "/"), nil
}

func deletedAtValue(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.UTC()
}

func encodeTrashCursor(deletedAt time.Time, id uuid.UUID) string {
	payload := fmt.Sprintf("%s|%s", deletedAt.UTC().Format(time.RFC3339Nano), id.String())
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func parseTrashCursor(cursor string) (time.Time, uuid.UUID, error) {
	if strings.TrimSpace(cursor) == "" {
		return time.Time{}, uuid.Nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, domain.ErrPersistence
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, domain.ErrPersistence
	}
	deletedAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, domain.ErrPersistence
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, domain.ErrPersistence
	}
	return deletedAt.UTC(), id, nil
}

func upgradeSQLiteTrashIndexes(db *gorm.DB) error {
	for _, statement := range []string{
		`DROP INDEX IF EXISTS filesystem.ux_filesystem_nodes_root_name`,
		`DROP INDEX IF EXISTS filesystem.ux_filesystem_nodes_child_name`,
		`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_nodes_root_name ON nodes(namespace_id, normalized_name) WHERE parent_id IS NULL AND state = 'ACTIVE'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS filesystem.ux_filesystem_nodes_child_name ON nodes(namespace_id, parent_id, normalized_name) WHERE parent_id IS NOT NULL AND state = 'ACTIVE'`,
		`CREATE INDEX IF NOT EXISTS filesystem.ix_filesystem_nodes_trash_entries ON nodes(namespace_id, deleted_at DESC, id DESC) WHERE state = 'TRASHED'`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			return wrapPersistence(err)
		}
	}
	return nil
}
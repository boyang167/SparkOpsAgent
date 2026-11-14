package persistence

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func (r *Repository) ListSharedNodes(
	ctx context.Context,
	organizationID uuid.UUID,
	principals []repository.PrincipalRef,
	filter repository.SharedNodeFilter,
	at time.Time,
) ([]repository.SharedNode, string, error) {
	if len(principals) == 0 {
		return []repository.SharedNode{}, "", nil
	}
	conditions, args := typedPrincipalConditions(principals)
	db := postgres.DB(ctx, r.db)
	nodeIDs := db.Table(nodeRow{}.TableName()+" AS n").
		Select("DISTINCT n.id").
		Joins("JOIN "+namespaceRow{}.TableName()+" AS ns ON ns.id = n.namespace_id").
		Joins("JOIN "+resourceGrantRow{}.TableName()+" AS g ON g.node_id = n.id").
		Where("ns.organization_id = ? AND g.organization_id = ?", organizationID, organizationID).
		Where("n.state = ? AND g.revoked_at IS NULL AND (g.expires_at IS NULL OR g.expires_at > ?)", model.NodeStateActive, at).
		Where(strings.Join(conditions, " OR "), args...)
	if filter.NamespaceID != uuid.Nil {
		nodeIDs = nodeIDs.Where("n.namespace_id = ?", filter.NamespaceID)
	}
	if needle := normalizedSearch(filter.Query); needle != "" {
		nodeIDs = nodeIDs.Where(`n.normalized_name LIKE ? ESCAPE '\'`, "%"+escapeLike(needle)+"%")
	}
	if filter.Cursor != "" {
		cursor, err := uuid.Parse(filter.Cursor)
		if err != nil || cursor == uuid.Nil {
			return nil, "", domain.ErrNodeNotFound
		}
		nodeIDs = nodeIDs.Where("n.id > ?", cursor)
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	var ids []uuid.UUID
	if err := nodeIDs.Order("n.id ASC").Limit(limit + 1).Scan(&ids).Error; err != nil {
		return nil, "", wrapPersistence(err)
	}
	next := ""
	if len(ids) > limit {
		next = ids[limit-1].String()
		ids = ids[:limit]
	}
	if len(ids) == 0 {
		return []repository.SharedNode{}, next, nil
	}
	var nodes []nodeRow
	if err := db.Where("id IN ?", ids).Find(&nodes).Error; err != nil {
		return nil, "", wrapPersistence(err)
	}
	var grants []resourceGrantRow
	if err := db.Table(resourceGrantRow{}.TableName()+" AS g").
		Where("g.node_id IN ? AND g.organization_id = ? AND g.revoked_at IS NULL AND (g.expires_at IS NULL OR g.expires_at > ?)", ids, organizationID, at).
		Where(strings.Join(conditions, " OR "), args...).
		Find(&grants).Error; err != nil {
		return nil, "", wrapPersistence(err)
	}
	grantsByNode := make(map[uuid.UUID][]resourceGrantRow, len(ids))
	for _, grant := range grants {
		grantsByNode[grant.NodeID] = append(grantsByNode[grant.NodeID], grant)
	}
	projections, err := sharedNodeProjections(ctx, db, organizationID, ids, principals, at)
	if err != nil {
		return nil, "", err
	}
	nodesByID := make(map[uuid.UUID]nodeRow, len(nodes))
	for _, node := range nodes {
		nodesByID[node.ID] = node
	}
	out := make([]repository.SharedNode, 0, len(ids))
	for _, id := range ids {
		node, exists := nodesByID[id]
		if !exists {
			continue
		}
		actions, sourceIDs := effectiveGrantActions(grantsByNode[id])
		projection := projections[id]
		out = append(out, repository.SharedNode{
			Node: *nodeFromRow(node), EffectiveActions: actions, SourceGrantIDs: sourceIDs,
			Sources: projection.Sources, OriginalPath: projection.OriginalPath,
			SourceNamespace: projection.SourceNamespace,
		})
	}
	return out, next, nil
}

type sharedNodeProjection struct {
	Sources         []repository.SharedGrantSource
	OriginalPath    string
	SourceNamespace repository.SharedNamespaceSource
}

func sharedNodeProjections(
	ctx context.Context,
	db *gorm.DB,
	organizationID uuid.UUID,
	nodeIDs []uuid.UUID,
	principals []repository.PrincipalRef,
	at time.Time,
) (map[uuid.UUID]sharedNodeProjection, error) {
	var ancestors []nodeRow
	sql := `WITH RECURSIVE ancestors AS (
		SELECT * FROM filesystem.nodes WHERE id IN ?
		UNION
		SELECT parent.* FROM filesystem.nodes parent
		JOIN ancestors child ON child.parent_id = parent.id
			AND child.namespace_id = parent.namespace_id
	) SELECT DISTINCT * FROM ancestors`
	if err := db.WithContext(ctx).Raw(sql, nodeIDs).Scan(&ancestors).Error; err != nil {
		return nil, wrapPersistence(err)
	}
	nodeByID := make(map[uuid.UUID]nodeRow, len(ancestors))
	ancestorIDs := make([]uuid.UUID, 0, len(ancestors))
	namespaceIDs := make([]uuid.UUID, 0, len(ancestors))
	namespaceSet := make(map[uuid.UUID]struct{})
	for _, node := range ancestors {
		nodeByID[node.ID] = node
		ancestorIDs = append(ancestorIDs, node.ID)
		if _, exists := namespaceSet[node.NamespaceID]; !exists {
			namespaceSet[node.NamespaceID] = struct{}{}
			namespaceIDs = append(namespaceIDs, node.NamespaceID)
		}
	}
	conditions, args := typedPrincipalConditions(principals)
	var grants []resourceGrantRow
	if len(ancestorIDs) > 0 {
		if err := db.Table(resourceGrantRow{}.TableName()+" AS g").
			Where("g.node_id IN ? AND g.organization_id = ? AND g.effect = ? AND g.revoked_at IS NULL AND (g.expires_at IS NULL OR g.expires_at > ?)",
				ancestorIDs, organizationID, model.GrantEffectAllow, at).
			Where(strings.Join(conditions, " OR "), args...).
			Find(&grants).Error; err != nil {
			return nil, wrapPersistence(err)
		}
	}
	grantsByNode := make(map[uuid.UUID][]resourceGrantRow)
	for _, grant := range grants {
		grantsByNode[grant.NodeID] = append(grantsByNode[grant.NodeID], grant)
	}
	var namespaces []namespaceRow
	if len(namespaceIDs) > 0 {
		if err := db.Where("id IN ? AND organization_id = ?", namespaceIDs, organizationID).
			Find(&namespaces).Error; err != nil {
			return nil, wrapPersistence(err)
		}
	}
	namespaceByID := make(map[uuid.UUID]namespaceRow, len(namespaces))
	for _, namespace := range namespaces {
		namespaceByID[namespace.ID] = namespace
	}
	out := make(map[uuid.UUID]sharedNodeProjection, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		node, exists := nodeByID[nodeID]
		if !exists {
			continue
		}
		namespace, namespaceExists := namespaceByID[node.NamespaceID]
		if !namespaceExists {
			continue
		}
		pathParts := make([]string, 0, 8)
		sources := make([]repository.SharedGrantSource, 0)
		visited := make(map[uuid.UUID]struct{})
		current := node
		for {
			if _, repeated := visited[current.ID]; repeated {
				break
			}
			visited[current.ID] = struct{}{}
			if namespace.RootNodeID == nil || current.ID != *namespace.RootNodeID {
				pathParts = append(pathParts, current.Name)
			}
			for _, grant := range grantsByNode[current.ID] {
				if current.ID != nodeID && !grant.InheritToChildren {
					continue
				}
				sources = append(sources, repository.SharedGrantSource{
					GrantID: grant.ID, PrincipalType: grant.PrincipalType,
					PrincipalID: grant.PrincipalID, ExpiresAt: grant.ExpiresAt,
				})
			}
			if current.ParentID == nil {
				break
			}
			parent, found := nodeByID[*current.ParentID]
			if !found || parent.NamespaceID != node.NamespaceID {
				break
			}
			current = parent
		}
		sort.Slice(sources, func(i, j int) bool {
			return sources[i].GrantID.String() < sources[j].GrantID.String()
		})
		for left, right := 0, len(pathParts)-1; left < right; left, right = left+1, right-1 {
			pathParts[left], pathParts[right] = pathParts[right], pathParts[left]
		}
		var ownerUserID *uuid.UUID
		if namespace.NamespaceType == model.NamespaceTypeUser && namespace.OwnerUserID != nil {
			value := *namespace.OwnerUserID
			ownerUserID = &value
		}
		var workspaceID *uuid.UUID
		if namespace.NamespaceType == model.NamespaceTypeWorkspace && namespace.WorkspaceID != nil {
			value := *namespace.WorkspaceID
			workspaceID = &value
		}
		out[nodeID] = sharedNodeProjection{
			Sources: sources, OriginalPath: "/" + strings.Join(pathParts, "/"),
			SourceNamespace: repository.SharedNamespaceSource{
				NamespaceType: namespace.NamespaceType, OrganizationID: namespace.OrganizationID,
				WorkspaceID: workspaceID, OwnerUserID: ownerUserID,
			},
		}
	}
	return out, nil
}

func typedPrincipalConditions(principals []repository.PrincipalRef) ([]string, []any) {
	conditions := make([]string, 0, len(principals))
	args := make([]any, 0, len(principals)*2)
	for _, principal := range principals {
		conditions = append(conditions, "(g.principal_type = ? AND g.principal_id = ?)")
		args = append(args, principal.Type, principal.ID)
	}
	return conditions, args
}

func effectiveGrantActions(grants []resourceGrantRow) ([]model.ResourcePermission, []uuid.UUID) {
	allowed := make(map[model.ResourcePermission][]uuid.UUID)
	denied := make(map[model.ResourcePermission]bool)
	for _, grant := range grants {
		if grant.Effect == model.GrantEffectDeny {
			denied[grant.Permission] = true
			continue
		}
		allowed[grant.Permission] = append(allowed[grant.Permission], grant.ID)
	}
	actions := []model.ResourcePermission{
		model.ResourcePermissionRead, model.ResourcePermissionDownload, model.ResourcePermissionWrite, model.ResourcePermissionDelete,
		model.ResourcePermissionShare, model.ResourcePermissionAdmin,
	}
	effective := make([]model.ResourcePermission, 0, len(actions))
	sourceSet := make(map[uuid.UUID]struct{})
	for _, action := range actions {
		if denied[model.ResourcePermissionAdmin] || denied[action] {
			continue
		}
		sources := append([]uuid.UUID{}, allowed[action]...)
		if action != model.ResourcePermissionAdmin {
			sources = append(sources, allowed[model.ResourcePermissionAdmin]...)
		}
		if len(sources) == 0 {
			continue
		}
		effective = append(effective, action)
		for _, source := range sources {
			sourceSet[source] = struct{}{}
		}
	}
	sourceIDs := make([]uuid.UUID, 0, len(sourceSet))
	for id := range sourceSet {
		sourceIDs = append(sourceIDs, id)
	}
	sort.Slice(sourceIDs, func(i, j int) bool { return sourceIDs[i].String() < sourceIDs[j].String() })
	return effective, sourceIDs
}

func normalizedSearch(value string) string {
	return strings.ToLower(strings.TrimSpace(norm.NFC.String(value)))
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func (r *Repository) GetNodesWithAncestors(ctx context.Context, namespaceID uuid.UUID, nodeIDs []uuid.UUID) ([]model.Node, error) {
	if len(nodeIDs) == 0 {
		return []model.Node{}, nil
	}
	var rows []nodeRow
	sql := `WITH RECURSIVE ancestors AS (
		SELECT * FROM filesystem.nodes WHERE namespace_id = ? AND id IN ?
		UNION
		SELECT parent.* FROM filesystem.nodes parent
		JOIN ancestors child ON child.parent_id = parent.id
		WHERE parent.namespace_id = ?
	) SELECT * FROM ancestors`
	if err := postgres.DB(ctx, r.db).Raw(sql, namespaceID, nodeIDs, namespaceID).Scan(&rows).Error; err != nil {
		return nil, wrapPersistence(err)
	}
	out := make([]model.Node, len(rows))
	for index := range rows {
		out[index] = *nodeFromRow(rows[index])
	}
	return out, nil
}

func (r *Repository) ListResourceGrantsForActions(
	ctx context.Context,
	nodeIDs []uuid.UUID,
	principals []repository.PrincipalRef,
	actions []model.ResourcePermission,
	at time.Time,
) ([]model.ResourceGrant, error) {
	if len(nodeIDs) == 0 || len(principals) == 0 || len(actions) == 0 {
		return []model.ResourceGrant{}, nil
	}
	conditions, args := typedPrincipalConditions(principals)
	var rows []resourceGrantRow
	if err := postgres.DB(ctx, r.db).Table(resourceGrantRow{}.TableName()+" AS g").
		Where("g.node_id IN ? AND g.permission IN ? AND g.revoked_at IS NULL AND (g.expires_at IS NULL OR g.expires_at > ?)", nodeIDs, actions, at).
		Where(strings.Join(conditions, " OR "), args...).
		Find(&rows).Error; err != nil {
		return nil, wrapPersistence(err)
	}
	out := make([]model.ResourceGrant, len(rows))
	for index, row := range rows {
		out[index] = resourceGrantFromRow(row)
	}
	return out, nil
}

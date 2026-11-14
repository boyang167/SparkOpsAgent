package persistence

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"

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
		if len(actions) == 0 {
			continue
		}
		out = append(out, repository.SharedNode{
			Node: *nodeFromRow(node), EffectiveActions: actions, SourceGrantIDs: sourceIDs,
		})
	}
	return out, next, nil
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
		model.ResourcePermissionRead, model.ResourcePermissionWrite, model.ResourcePermissionDelete,
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

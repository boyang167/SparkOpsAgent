package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const governanceContextKey = "filesystem_governance_context"

type GovernanceRepository interface {
	GetNamespace(context.Context, uuid.UUID) (*model.Namespace, error)
	GetNode(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	GetNodeByID(context.Context, uuid.UUID) (*model.Node, error)
	ListChildrenFiltered(context.Context, uuid.UUID, *uuid.UUID, *model.NodeType, string, int, string) ([]model.Node, string, error)
}

type GovernanceHandler struct {
	namespaceByID   func(context.Context, uuid.UUID) (*model.Namespace, error)
	nodeByID        func(context.Context, uuid.UUID) (*model.Node, error)
	getNode         func(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	listChildren    func(context.Context, uuid.UUID, *uuid.UUID, *model.NodeType, string, int, string) ([]model.Node, string, error)
	listNamespaces  func(context.Context, query.ListNamespaces) ([]query.NamespaceSummary, error)
	sharedWithMe    func(context.Context, query.SharedWithMe) ([]repository.SharedNode, string, error)
	effectiveAccess func(context.Context, query.EffectiveAccess) query.EffectiveAccessResult
	authorize       func(context.Context, resourceauth.Request) resourceauth.Decision
}

func NewGovernanceHandler(
	repo GovernanceRepository,
	namespaces *query.ListNamespacesHandler,
	shared *query.SharedWithMeHandler,
	effective *query.EffectiveAccessHandler,
	authorizer query.ResourceAuthorizer,
) *GovernanceHandler {
	return &GovernanceHandler{
		namespaceByID:   repo.GetNamespace,
		nodeByID:        repo.GetNodeByID,
		getNode:         repo.GetNode,
		listChildren:    repo.ListChildrenFiltered,
		listNamespaces:  namespaces.Handle,
		sharedWithMe:    shared.Handle,
		effectiveAccess: effective.Handle,
		authorize:       authorizer.Authorize,
	}
}

func RegisterGovernanceRoutes(
	engine *gin.Engine,
	entitlement gin.HandlerFunc,
	auth gin.HandlerFunc,
	organizations authz.OrgPermissionChecker,
	workspaces authz.WorkspaceAccessResolver,
	handler *GovernanceHandler,
) {
	base := engine.Group("/api/v1/filesystem")
	base.Use(auth, governanceContextMiddleware(organizations, workspaces))
	if entitlement != nil {
		base.Use(entitlement)
	}
	base.GET("/namespaces", handler.ListNamespaces)
	base.GET("/namespaces/:namespaceId/children", handler.ListChildren)
	base.GET("/shared-with-me", handler.SharedWithMe)
	base.GET("/nodes/:nodeId/effective-access", handler.EffectiveAccess)
}

type governanceContext struct {
	organizationID uuid.UUID
	workspaceID    uuid.UUID
	actorID        uuid.UUID
}

func governanceContextMiddleware(organizations authz.OrgPermissionChecker, workspaces authz.WorkspaceAccessResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, ok := authn.MemberIDFromContext(c)
		if !ok {
			httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
			c.Abort()
			return
		}
		rawOrg := c.GetHeader("X-Organization-ID")
		if rawOrg == "" {
			httpx.Fail(c, "TENANT_ORG_REQUIRED", "X-Organization-ID header required", http.StatusBadRequest)
			c.Abort()
			return
		}
		organizationID, ok := canonicalUUID(rawOrg)
		if !ok {
			httpx.Fail(c, "TENANT_ORG_INVALID", "invalid X-Organization-ID header", http.StatusBadRequest)
			c.Abort()
			return
		}
		if organizations == nil {
			httpx.Fail(c, "TENANT_ORG_ACCESS_DENIED", "organization membership required", http.StatusForbidden)
			c.Abort()
			return
		}
		member, err := organizations.IsOrgMember(c.Request.Context(), kernel.OrgID{UUID: organizationID}, actorID)
		if err != nil {
			httpx.HandleError(c, err)
			c.Abort()
			return
		}
		if !member {
			httpx.Fail(c, "TENANT_ORG_ACCESS_DENIED", "organization membership required", http.StatusForbidden)
			c.Abort()
			return
		}
		contextValue := governanceContext{organizationID: organizationID, actorID: actorID.UUID}
		if rawWorkspace := c.GetHeader("X-Workspace-ID"); rawWorkspace != "" {
			workspaceID, valid := canonicalUUID(rawWorkspace)
			if !valid {
				httpx.Fail(c, "TENANT_WORKSPACE_INVALID", "invalid X-Workspace-ID header", http.StatusBadRequest)
				c.Abort()
				return
			}
			if workspaces == nil {
				httpx.Fail(c, "TENANT_NOT_WORKSPACE_MEMBER", "workspace membership required", http.StatusForbidden)
				c.Abort()
				return
			}
			resolvedOrg, active, resolveErr := workspaces.ResolveWorkspaceAccess(
				c.Request.Context(), kernel.WorkspaceID{UUID: workspaceID}, actorID,
			)
			if resolveErr != nil {
				httpx.HandleError(c, resolveErr)
				c.Abort()
				return
			}
			if !active || resolvedOrg.UUID != organizationID {
				httpx.Fail(c, "TENANT_NOT_WORKSPACE_MEMBER", "workspace membership required", http.StatusForbidden)
				c.Abort()
				return
			}
			contextValue.workspaceID = workspaceID
		}
		c.Set(governanceContextKey, contextValue)
		c.Next()
	}
}

func governanceRequestContext(c *gin.Context) (governanceContext, bool) {
	value, ok := c.Get(governanceContextKey)
	if !ok {
		return governanceContext{}, false
	}
	result, ok := value.(governanceContext)
	return result, ok
}

func (h *GovernanceHandler) ListNamespaces(c *gin.Context) {
	rc, ok := governanceRequestContext(c)
	if !ok || h == nil || h.listNamespaces == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	items, err := h.listNamespaces(c.Request.Context(), query.ListNamespaces{
		ActorID: rc.actorID, OrganizationID: rc.organizationID, WorkspaceID: rc.workspaceID,
	})
	if err != nil {
		governanceError(c, err)
		return
	}
	views := make([]gin.H, 0, len(items))
	for _, item := range items {
		views = append(views, namespaceSummaryView(item))
	}
	httpx.OK(c, gin.H{"items": views})
}

func (h *GovernanceHandler) ListChildren(c *gin.Context) {
	rc, namespace, ok := h.namespaceContext(c)
	if !ok {
		return
	}
	parentID, ok := optionalUUIDQuery(c, "parent_id")
	if !ok {
		return
	}
	explicitParent := parentID != nil
	if parentID == nil {
		parentID = namespace.RootNodeID
	}
	if parentID == nil {
		notFound(c)
		return
	}
	if h.getNode == nil || h.listChildren == nil || h.authorize == nil {
		governanceError(c, domain.ErrPersistence)
		return
	}
	parent, err := h.getNode(c.Request.Context(), namespace.ID, *parentID)
	if err != nil {
		governanceError(c, err)
		return
	}
	if parent.Type != model.NodeTypeDirectory {
		notFound(c)
		return
	}
	if explicitParent && !h.authorize(c.Request.Context(), resourceauth.Request{
		ActorID: rc.actorID, WorkspaceID: rc.workspaceID, NamespaceID: namespace.ID,
		NodeID: *parentID, Permission: model.ResourcePermissionRead,
	}).Allowed {
		notFound(c)
		return
	}
	limit, ok := pagination(c)
	if !ok {
		return
	}
	nodeType, ok := optionalNodeTypeQuery(c)
	if !ok {
		return
	}
	search := strings.TrimSpace(c.Query("q"))
	if len(search) > 200 {
		invalidRequest(c)
		return
	}
	visible := make([]model.Node, 0, limit+1)
	scanCursor := c.Query("cursor")
	for len(visible) <= limit {
		pageStart := scanCursor
		nodes, rawNext, listErr := h.listChildren(c.Request.Context(), namespace.ID, parentID, nodeType, search, 200, scanCursor)
		if listErr != nil {
			governanceError(c, listErr)
			return
		}
		for index := range nodes {
			scanCursor = nodes[index].ID.String()
			if h.authorize(c.Request.Context(), resourceauth.Request{
				ActorID: rc.actorID, WorkspaceID: rc.workspaceID, NamespaceID: namespace.ID,
				NodeID: nodes[index].ID, Permission: model.ResourcePermissionRead,
			}).Allowed {
				visible = append(visible, nodes[index])
				if len(visible) == limit+1 {
					break
				}
			}
		}
		if len(visible) == limit+1 || rawNext == "" || rawNext == pageStart {
			break
		}
		scanCursor = rawNext
	}
	next := ""
	if len(visible) > limit {
		next = visible[limit-1].ID.String()
		visible = visible[:limit]
	}
	items := make([]gin.H, 0, len(visible))
	for index := range visible {
		view := nodeView(&visible[index])
		if !explicitParent && visible[index].ParentID != nil && *visible[index].ParentID == *parentID {
			view["parent_id"] = nil
		}
		items = append(items, view)
	}
	httpx.OK(c, gin.H{"items": items, "next_cursor": next})
}

func (h *GovernanceHandler) SharedWithMe(c *gin.Context) {
	rc, ok := governanceRequestContext(c)
	if !ok || h == nil || h.sharedWithMe == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	limit, ok := governancePagination(c)
	if !ok {
		return
	}
	search := strings.TrimSpace(c.Query("q"))
	if len(search) > 200 {
		invalidRequest(c)
		return
	}
	items, next, err := h.sharedWithMe(c.Request.Context(), query.SharedWithMe{
		ActorID: rc.actorID, OrganizationID: rc.organizationID, WorkspaceID: rc.workspaceID,
		Query: search, Cursor: c.Query("cursor"), Limit: limit,
	})
	if err != nil {
		governanceError(c, err)
		return
	}
	views := make([]gin.H, 0, len(items))
	for _, item := range items {
		views = append(views, sharedNodeView(item))
	}
	httpx.OK(c, gin.H{"items": views, "next_cursor": next})
}

func (h *GovernanceHandler) EffectiveAccess(c *gin.Context) {
	rc, ok := governanceRequestContext(c)
	if !ok || h == nil || h.nodeByID == nil || h.effectiveAccess == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	nodeID, ok := canonicalUUID(c.Param("nodeId"))
	if !ok {
		invalidRequest(c)
		return
	}
	node, err := h.nodeByID(c.Request.Context(), nodeID)
	if err != nil {
		governanceError(c, err)
		return
	}
	namespace, err := h.namespaceByID(c.Request.Context(), node.NamespaceID)
	if err != nil {
		governanceError(c, err)
		return
	}
	if namespace == nil {
		governanceError(c, domain.ErrPersistence)
		return
	}
	if !validNamespaceAudience(rc, namespace) {
		if err == nil && namespace != nil && namespace.OrganizationID == rc.organizationID && namespace.NamespaceType != model.NamespaceTypeUser && rc.workspaceID == uuid.Nil {
			httpx.Fail(c, "TENANT_NOT_WORKSPACE_MEMBER", "workspace context required", http.StatusForbidden)
			return
		}
		notFound(c)
		return
	}
	result := h.effectiveAccess(c.Request.Context(), query.EffectiveAccess{
		ActorID: rc.actorID, WorkspaceID: rc.workspaceID, NamespaceID: namespace.ID, NodeID: nodeID,
	})
	httpx.OK(c, effectiveAccessView(result))
}

func (h *GovernanceHandler) namespaceContext(c *gin.Context) (governanceContext, *model.Namespace, bool) {
	rc, ok := governanceRequestContext(c)
	if !ok || h == nil || h.namespaceByID == nil {
		filesystemError(c, domain.ErrPersistence)
		return governanceContext{}, nil, false
	}
	namespaceID, valid := canonicalUUID(c.Param("namespaceId"))
	if !valid {
		invalidRequest(c)
		return governanceContext{}, nil, false
	}
	namespace, err := h.namespaceByID(c.Request.Context(), namespaceID)
	if err != nil {
		governanceError(c, err)
		return governanceContext{}, nil, false
	}
	if namespace == nil {
		governanceError(c, domain.ErrPersistence)
		return governanceContext{}, nil, false
	}
	if namespace.OrganizationID != rc.organizationID {
		notFound(c)
		return governanceContext{}, nil, false
	}
	if namespace.NamespaceType != model.NamespaceTypeUser && rc.workspaceID == uuid.Nil {
		httpx.Fail(c, "TENANT_NOT_WORKSPACE_MEMBER", "workspace context required", http.StatusForbidden)
		return governanceContext{}, nil, false
	}
	if !validNamespaceAudience(rc, namespace) {
		notFound(c)
		return governanceContext{}, nil, false
	}
	return rc, namespace, true
}

func validNamespaceAudience(rc governanceContext, namespace *model.Namespace) bool {
	if namespace == nil || namespace.OrganizationID != rc.organizationID {
		return false
	}
	switch namespace.NamespaceType {
	case model.NamespaceTypeUser:
		return namespace.OwnerUserID == rc.actorID
	case model.NamespaceTypeWorkspace:
		return rc.workspaceID != uuid.Nil && namespace.WorkspaceID == rc.workspaceID
	case model.NamespaceTypeOrganization:
		return rc.workspaceID != uuid.Nil
	default:
		return false
	}
}

func governanceError(c *gin.Context, err error) {
	if errors.Is(err, resourceauth.ErrPrincipalScopeMismatch) {
		httpx.Fail(c, "TENANT_NOT_WORKSPACE_MEMBER", "workspace membership required", http.StatusForbidden)
		return
	}
	switch {
	case errors.Is(err, domain.ErrInvalidGovernanceCursor):
		httpx.Fail(c, "FILESYSTEM_INVALID_REQUEST", "invalid governance cursor", http.StatusBadRequest)
	case errors.Is(err, domain.ErrAuthorizationScanBudgetExceeded):
		httpx.Fail(c, "FILESYSTEM_AUTHORIZATION_SCAN_BUDGET_EXCEEDED", "authorization scan budget exceeded", http.StatusServiceUnavailable)
	case errors.Is(err, domain.ErrResourceDenied):
		httpx.Fail(c, "FILESYSTEM_RESOURCE_FORBIDDEN", "filesystem resource access denied", http.StatusForbidden)
	case errors.Is(err, domain.ErrNamespaceNotFound), errors.Is(err, domain.ErrNodeNotFound):
		notFound(c)
	default:
		httpx.Fail(c, "FILESYSTEM_INTERNAL_ERROR", "filesystem operation failed", http.StatusInternalServerError)
	}
}

func notFound(c *gin.Context) {
	httpx.Fail(c, "FILESYSTEM_NOT_FOUND", "filesystem resource not found", http.StatusNotFound)
}

func namespaceSummaryView(item query.NamespaceSummary) gin.H {
	return gin.H{
		"id": item.ID, "namespace_type": item.NamespaceType, "organization_id": item.OrganizationID,
		"workspace_id": item.WorkspaceID, "owner_user_id": item.OwnerUserID, "root_node_id": item.RootNodeID,
		"status": item.Status, "capabilities": item.Capabilities, "entry_node_ids": item.EntryNodeIDs,
		"entry_nodes_truncated": item.EntryNodesTruncated,
	}
}

func governancePagination(c *gin.Context) (int, bool) {
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 200 {
			invalidRequest(c)
			return 0, false
		}
		limit = value
	}
	if len(c.Query("cursor")) > 512 {
		invalidRequest(c)
		return 0, false
	}
	return limit, true
}

func sharedNodeView(item repository.SharedNode) gin.H {
	return gin.H{
		"node": nodeView(&item.Node), "effective_actions": item.EffectiveActions,
		"source_grant_ids": item.SourceGrantIDs,
	}
}

func effectiveAccessView(result query.EffectiveAccessResult) gin.H {
	return gin.H{
		"decision": result.Decision, "effective_actions": result.EffectiveActions,
		"reason_code": result.ReasonCode, "source_grant_ids": result.SourceGrantIDs,
		"policy_version": result.PolicyVersion,
		"cache_key": gin.H{
			"organization_id": result.CacheKey.OrganizationID, "workspace_id": result.CacheKey.WorkspaceID,
			"actor_id": result.CacheKey.ActorID, "node_id": result.CacheKey.NodeID,
			"action": result.CacheKey.Action, "policy_version": result.CacheKey.PolicyVersion,
		},
	}
}

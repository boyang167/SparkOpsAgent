package http

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const maxJSONBodyBytes = 64 << 10

type TransferOptions struct {
	Create  func(context.Context, command.CreateTransfer) (command.TransferDescriptor, error)
	Get     func(context.Context, query.GetTransfer) (query.GetTransferResult, error)
	Abort   func(context.Context, command.AbortTransfer) error
	Peek    func(context.Context, uuid.UUID) (*model.TransferSession, error)
	Gateway TransferGatewayView
}

type Handler struct {
	namespaceForWorkspace func(context.Context, uuid.UUID) (*model.Namespace, error)
	getNode               func(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
	listChildren          func(context.Context, uuid.UUID, *uuid.UUID, int, string) ([]model.Node, string, error)
	listRevisions         func(context.Context, uuid.UUID, uuid.UUID, int, string) ([]model.FileRevision, string, error)
	createDirectory       func(context.Context, command.CreateDirectory) (*model.Node, error)
	renameNode            func(context.Context, command.RenameNode) error
	moveNode              func(context.Context, command.MoveNode) error
	copyNode              func(context.Context, command.CopyNode) (*model.Node, error)
	trashNode             func(context.Context, command.TrashNode) error
	restoreRevision       func(context.Context, command.RestoreRevision) error
	authorizeResource     func(context.Context, resourceauth.Request) resourceauth.Decision
	audit                 audit.Logger
	createTransfer        func(context.Context, command.CreateTransfer) (command.TransferDescriptor, error)
	getTransfer           func(context.Context, query.GetTransfer) (query.GetTransferResult, error)
	abortTransfer         func(context.Context, command.AbortTransfer) error
	peekTransfer          func(context.Context, uuid.UUID) (*model.TransferSession, error)
	transferGatewayView   TransferGatewayView
}

func NewHandler(
	repo repository.FilesystemRepository,
	list *query.ListChildrenHandler,
	revisions *query.ListRevisionsHandler,
	create *command.CreateDirectoryHandler,
	rename *command.RenameNodeHandler,
	move *command.MoveNodeHandler,
	copyHandler *command.CopyNodeHandler,
	trash *command.TrashNodeHandler,
	restore *command.RestoreRevisionHandler,
	authorizer *resourceauth.Service,
	logger audit.Logger,
	transfers *TransferOptions,
) *Handler {
	handler := &Handler{
		namespaceForWorkspace: repo.GetNamespaceByWorkspace,
		getNode:               repo.GetNode,
		listChildren: func(ctx context.Context, namespaceID uuid.UUID, parentID *uuid.UUID, limit int, cursor string) ([]model.Node, string, error) {
			return list.Handle(ctx, query.ListChildren{NamespaceID: namespaceID, ParentID: parentID, Limit: limit, Cursor: cursor})
		},
		listRevisions: func(ctx context.Context, namespaceID, nodeID uuid.UUID, limit int, cursor string) ([]model.FileRevision, string, error) {
			return revisions.Handle(ctx, query.ListRevisions{NamespaceID: namespaceID, NodeID: nodeID, Limit: limit, Cursor: cursor})
		},
		createDirectory:   create.Handle,
		renameNode:        rename.Handle,
		moveNode:          move.Handle,
		copyNode:          copyHandler.Handle,
		trashNode:         trash.Handle,
		restoreRevision:   restore.Handle,
		authorizeResource: authorizer.Authorize,
		audit:             logger,
	}
	if transfers != nil {
		handler.createTransfer = transfers.Create
		handler.getTransfer = transfers.Get
		handler.abortTransfer = transfers.Abort
		handler.peekTransfer = transfers.Peek
		handler.transferGatewayView = transfers.Gateway
	}
	return handler
}

func RegisterRoutes(engine *gin.Engine, entitlement gin.HandlerFunc, auth gin.HandlerFunc, members authz.WorkspaceMemberChecker, permissions authz.PermissionChecker, h *Handler) {
	base := engine.Group("/api/v1/workspaces/:wsId")
	base.Use(auth, httpx.RequireWorkspace(), requireCanonicalWorkspacePath(), authz.RequireWorkspaceMember(members))
	if entitlement != nil {
		base.Use(entitlement)
	}
	base.Use(h.auditMiddlewareDenial())
	base.GET("/namespaces", authz.RequirePermission(permissions, authz.PermFileRead), h.GetNamespace)
	base.GET("/files", authz.RequirePermission(permissions, authz.PermFileRead), h.ListFiles)
	base.POST("/directories", authz.RequirePermission(permissions, authz.PermFileWrite), requireIdempotencyKey(), h.CreateDirectory)
	base.PATCH("/files/:nodeId/name", authz.RequirePermission(permissions, authz.PermFileWrite), requireIdempotencyKey(), h.RenameNode)
	base.POST("/files/:nodeId/move", authz.RequirePermission(permissions, authz.PermFileWrite), requireIdempotencyKey(), h.MoveNode)
	base.POST("/files/:nodeId/copy", authz.RequirePermission(permissions, authz.PermFileWrite), requireIdempotencyKey(), h.CopyNode)
	base.DELETE("/files/:nodeId", authz.RequirePermission(permissions, authz.PermFileDelete), requireIdempotencyKey(), h.TrashNode)
	base.GET("/files/:nodeId/revisions", authz.RequirePermission(permissions, authz.PermFileRead), h.ListRevisions)
	base.POST("/files/:nodeId/revisions/:revisionId/restore", authz.RequirePermission(permissions, authz.PermFileWrite), requireIdempotencyKey(), h.RestoreRevision)
	base.POST("/transfers", authz.RequirePermission(permissions, authz.PermFileWrite), requireIdempotencyKey(), h.CreateTransfer)
	base.GET("/transfers/:sessionId", authz.RequirePermission(permissions, authz.PermFileRead), h.GetTransfer)
	base.DELETE("/transfers/:sessionId", authz.RequirePermission(permissions, authz.PermFileWrite), requireIdempotencyKey(), h.AbortTransfer)
}

func (h *Handler) auditMiddlewareDenial() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if h == nil || h.audit == nil || !c.IsAborted() || c.Writer.Status() != http.StatusForbidden {
			return
		}
		workspaceID, workspaceOK := httpx.WorkspaceIDFromContext(c)
		actorID, actorOK := authn.MemberIDFromContext(c)
		if !workspaceOK || !actorOK {
			return
		}
		resourceID := c.Param("nodeId")
		if resourceID == "" {
			resourceID = workspaceID.String()
		}
		var orgID *kernel.OrgID
		if h.namespaceForWorkspace != nil {
			if namespace, err := h.namespaceForWorkspace(c.Request.Context(), workspaceID.UUID); err == nil && namespace.WorkspaceID == workspaceID.UUID {
				orgID = &kernel.OrgID{UUID: namespace.OrganizationID}
			}
		}
		_ = h.audit.Log(c.Request.Context(), audit.Entry{
			ActorID: actorID, Action: "filesystem.middleware.authorize", ResourceType: "filesystem",
			ResourceID: resourceID, OrgID: orgID,
			Detail: map[string]any{"workspace_id": workspaceID.String(), "decision": "DENY", "reason": "middleware_denied", "trace_id": httpx.GetTraceID(c)},
		})
	}
}

func requireCanonicalWorkspacePath() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := canonicalUUID(c.Param("wsId")); !ok {
			invalidRequest(c)
			c.Abort()
			return
		}
		c.Next()
	}
}

const idempotencyContextKey = "filesystem_idempotency_key"

func requireIdempotencyKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if key == "" {
			httpx.Fail(c, "FILESYSTEM_IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header required", http.StatusBadRequest)
			c.Abort()
			return
		}
		if len(key) > 128 || strings.ContainsAny(key, "\r\n\t") {
			httpx.Fail(c, "FILESYSTEM_IDEMPOTENCY_KEY_INVALID", "invalid Idempotency-Key header", http.StatusBadRequest)
			c.Abort()
			return
		}
		c.Set(idempotencyContextKey, key)
		c.Next()
	}
}

func commandID(workspaceID uuid.UUID, key string) uuid.UUID {
	sum := sha256.Sum256([]byte(workspaceID.String() + "\x00" + key))
	return uuid.NewSHA1(uuid.NameSpaceURL, sum[:])
}

type requestContext struct {
	workspaceID uuid.UUID
	actorID     uuid.UUID
	namespace   *model.Namespace
	commandID   uuid.UUID
	traceID     string
}

func (h *Handler) resourceDecision(c *gin.Context, rc requestContext, nodeID uuid.UUID, permission model.ResourcePermission) command.Authorization {
	if h.authorizeResource == nil {
		return command.Authorization{Reason: "authorization_unavailable", TraceID: rc.traceID}
	}
	decision := h.authorizeResource(c.Request.Context(), resourceauth.Request{
		ActorID: rc.actorID, WorkspaceID: rc.workspaceID, NamespaceID: rc.namespace.ID,
		NodeID: nodeID, Permission: permission,
	})
	return command.Authorization{Allowed: decision.Allowed, Reason: decision.Reason, TraceID: rc.traceID}
}

func (h *Handler) requireResource(c *gin.Context, rc requestContext, nodeID uuid.UUID, permission model.ResourcePermission) bool {
	decision := h.resourceDecision(c, rc, nodeID, permission)
	if decision.Allowed {
		return true
	}
	h.auditResourceDeny(c, rc, nodeID, decision.Reason)
	filesystemError(c, domain.ErrResourceDenied)
	return false
}

func (h *Handler) auditResourceDeny(c *gin.Context, rc requestContext, nodeID uuid.UUID, reason string) {
	if h.audit != nil {
		_ = h.audit.Log(c.Request.Context(), audit.Entry{
			ActorID: kernel.MemberID{UUID: rc.actorID}, Action: "filesystem.resource.authorize",
			ResourceType: "filesystem", ResourceID: nodeID.String(),
			OrgID:  &kernel.OrgID{UUID: rc.namespace.OrganizationID},
			Detail: map[string]any{"workspace_id": rc.workspaceID.String(), "decision": "DENY", "reason": reason, "trace_id": rc.traceID},
		})
	}
}

func (h *Handler) requestContext(c *gin.Context, mutation bool) (requestContext, bool) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return requestContext{}, false
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return requestContext{}, false
	}
	if h == nil || h.namespaceForWorkspace == nil {
		filesystemError(c, domain.ErrPersistence)
		return requestContext{}, false
	}
	namespace, err := h.namespaceForWorkspace(c.Request.Context(), wsID.UUID)
	if err != nil {
		filesystemError(c, err)
		return requestContext{}, false
	}
	if namespace.WorkspaceID != wsID.UUID {
		filesystemError(c, domain.ErrNamespaceNotFound)
		return requestContext{}, false
	}
	out := requestContext{workspaceID: wsID.UUID, actorID: memberID.UUID, namespace: namespace, traceID: httpx.GetTraceID(c)}
	if mutation {
		key, _ := c.Get(idempotencyContextKey)
		out.commandID = commandID(wsID.UUID, key.(string))
	}
	return out, true
}

func (h *Handler) GetNamespace(c *gin.Context) {
	rc, ok := h.requestContext(c, false)
	if !ok {
		return
	}
	if rc.namespace.RootNodeID == nil || !h.requireResource(c, rc, *rc.namespace.RootNodeID, model.ResourcePermissionRead) {
		return
	}
	httpx.OK(c, namespaceView(rc.namespace))
}

func (h *Handler) ListFiles(c *gin.Context) {
	rc, ok := h.requestContext(c, false)
	if !ok {
		return
	}
	parentID, ok := optionalUUIDQuery(c, "parent_id")
	if !ok {
		return
	}
	limit, ok := pagination(c)
	if !ok {
		return
	}
	if h.listChildren == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	if parentID == nil {
		if rc.namespace.RootNodeID == nil {
			filesystemError(c, domain.ErrNamespaceNotFound)
			return
		}
		parentID = rc.namespace.RootNodeID
	} else {
		if h.getNode == nil {
			filesystemError(c, domain.ErrPersistence)
			return
		}
		parent, err := h.getNode(c.Request.Context(), rc.namespace.ID, *parentID)
		if err != nil || parent.NamespaceID != rc.namespace.ID {
			if err == nil {
				err = domain.ErrNamespaceMismatch
			}
			filesystemError(c, err)
			return
		}
		if parent.Type != model.NodeTypeDirectory {
			filesystemError(c, domain.ErrParentNotDirectory)
			return
		}
	}
	if !h.requireResource(c, rc, *parentID, model.ResourcePermissionRead) {
		return
	}
	views := make([]gin.H, 0, limit)
	scanCursor := c.Query("cursor")
	nextCursor := ""
	for len(views) < limit {
		items, rawNext, err := h.listChildren(c.Request.Context(), rc.namespace.ID, parentID, 200, scanCursor)
		if err != nil {
			filesystemError(c, err)
			return
		}
		for i := range items {
			scanCursor = items[i].ID.String()
			decision := h.resourceDecision(c, rc, items[i].ID, model.ResourcePermissionRead)
			if !decision.Allowed {
				h.auditResourceDeny(c, rc, items[i].ID, decision.Reason)
				continue
			}
			views = append(views, nodeView(&items[i]))
			if len(views) == limit {
				if i < len(items)-1 || rawNext != "" {
					nextCursor = scanCursor
				}
				break
			}
		}
		if len(views) == limit || rawNext == "" {
			break
		}
		scanCursor = rawNext
	}
	httpx.OK(c, gin.H{"items": views, "next_cursor": nextCursor})
}

type createDirectoryRequest struct {
	NamespaceID string  `json:"namespace_id"`
	ParentID    *string `json:"parent_id"`
	Name        string  `json:"name"`
}

func (h *Handler) CreateDirectory(c *gin.Context) {
	rc, ok := h.requestContext(c, true)
	if !ok {
		return
	}
	var body createDirectoryRequest
	if !decodeJSON(c, &body) {
		return
	}
	if body.NamespaceID != "" {
		namespaceID, valid := canonicalUUID(body.NamespaceID)
		if !valid {
			invalidRequest(c)
			return
		}
		if namespaceID != rc.namespace.ID {
			filesystemError(c, domain.ErrNamespaceMismatch)
			return
		}
	}
	parentID, ok := optionalUUID(body.ParentID)
	if !ok || strings.TrimSpace(body.Name) == "" {
		invalidRequest(c)
		return
	}
	if h.createDirectory == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	if parentID == nil {
		parentID = rc.namespace.RootNodeID
	}
	if parentID == nil {
		filesystemError(c, domain.ErrNamespaceNotFound)
		return
	}
	authorization := h.resourceDecision(c, rc, *parentID, model.ResourcePermissionWrite)
	node, err := h.createDirectory(c.Request.Context(), command.CreateDirectory{
		CommandID: rc.commandID, ActorID: rc.actorID, OrganizationID: rc.namespace.OrganizationID,
		WorkspaceID: rc.workspaceID, NamespaceID: rc.namespace.ID, ParentID: parentID, Name: body.Name,
		Authorization: authorization,
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	created(c, nodeView(node))
}

type renameRequest struct {
	Name string `json:"name"`
}

func (h *Handler) RenameNode(c *gin.Context) {
	h.withNodeMutation(c, func(rc requestContext, nodeID uuid.UUID) error {
		var body renameRequest
		if !decodeJSON(c, &body) || strings.TrimSpace(body.Name) == "" {
			return errResponseWritten
		}
		if h.renameNode == nil {
			return domain.ErrPersistence
		}
		return h.renameNode(c.Request.Context(), command.RenameNode{CommandID: rc.commandID, ActorID: rc.actorID, OrganizationID: rc.namespace.OrganizationID, WorkspaceID: rc.workspaceID, NamespaceID: rc.namespace.ID, NodeID: nodeID, Name: body.Name, Authorization: h.resourceDecision(c, rc, nodeID, model.ResourcePermissionWrite)})
	})
}

type moveRequest struct {
	ParentID *string `json:"parent_id"`
	Name     string  `json:"name"`
}

func (h *Handler) MoveNode(c *gin.Context) {
	h.withNodeMutation(c, func(rc requestContext, nodeID uuid.UUID) error {
		var body moveRequest
		if !decodeJSON(c, &body) {
			return errResponseWritten
		}
		parentID, ok := optionalUUID(body.ParentID)
		if !ok {
			invalidRequest(c)
			return errResponseWritten
		}
		if h.moveNode == nil {
			return domain.ErrPersistence
		}
		targetParent := parentID
		if targetParent == nil {
			targetParent = rc.namespace.RootNodeID
		}
		authorization := h.resourceDecision(c, rc, nodeID, model.ResourcePermissionWrite)
		if authorization.Allowed && targetParent != nil {
			authorization = h.resourceDecision(c, rc, *targetParent, model.ResourcePermissionWrite)
		}
		return h.moveNode(c.Request.Context(), command.MoveNode{CommandID: rc.commandID, ActorID: rc.actorID, OrganizationID: rc.namespace.OrganizationID, WorkspaceID: rc.workspaceID, NamespaceID: rc.namespace.ID, NodeID: nodeID, NewParentID: targetParent, NewName: body.Name, Authorization: authorization})
	})
}

type copyRequest struct {
	ParentID *string `json:"parent_id"`
	Name     string  `json:"name"`
}

func (h *Handler) CopyNode(c *gin.Context) {
	rc, nodeID, ok := h.nodeRequest(c)
	if !ok {
		return
	}
	var body copyRequest
	if !decodeJSON(c, &body) {
		return
	}
	parentID, valid := optionalUUID(body.ParentID)
	if !valid {
		invalidRequest(c)
		return
	}
	if h.copyNode == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	targetParent := parentID
	if targetParent == nil {
		targetParent = rc.namespace.RootNodeID
	}
	authorization := h.resourceDecision(c, rc, nodeID, model.ResourcePermissionRead)
	if authorization.Allowed && targetParent != nil {
		authorization = h.resourceDecision(c, rc, *targetParent, model.ResourcePermissionWrite)
	}
	node, err := h.copyNode(c.Request.Context(), command.CopyNode{CommandID: rc.commandID, ActorID: rc.actorID, OrganizationID: rc.namespace.OrganizationID, WorkspaceID: rc.workspaceID, NamespaceID: rc.namespace.ID, SourceNodeID: nodeID, ParentID: targetParent, Name: body.Name, Authorization: authorization})
	if err != nil {
		filesystemError(c, err)
		return
	}
	created(c, nodeView(node))
}

func (h *Handler) TrashNode(c *gin.Context) {
	rc, nodeID, ok := h.nodeRequest(c)
	if !ok {
		return
	}
	if !requireEmptyBody(c) {
		return
	}
	if h.trashNode == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	err := h.trashNode(c.Request.Context(), command.TrashNode{CommandID: rc.commandID, ActorID: rc.actorID, OrganizationID: rc.namespace.OrganizationID, WorkspaceID: rc.workspaceID, NamespaceID: rc.namespace.ID, NodeID: nodeID, Authorization: h.resourceDecision(c, rc, nodeID, model.ResourcePermissionDelete)})
	if err != nil {
		filesystemError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListRevisions(c *gin.Context) {
	rc, ok := h.requestContext(c, false)
	if !ok {
		return
	}
	nodeID, ok := canonicalUUID(c.Param("nodeId"))
	if !ok {
		invalidRequest(c)
		return
	}
	limit, ok := pagination(c)
	if !ok {
		return
	}
	if h.listRevisions == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	if !h.requireResource(c, rc, nodeID, model.ResourcePermissionRead) {
		return
	}
	items, next, err := h.listRevisions(c.Request.Context(), rc.namespace.ID, nodeID, limit, c.Query("cursor"))
	if err != nil {
		filesystemError(c, err)
		return
	}
	views := make([]gin.H, 0, len(items))
	for i := range items {
		views = append(views, revisionView(&items[i]))
	}
	httpx.OK(c, gin.H{"items": views, "next_cursor": next})
}

func (h *Handler) RestoreRevision(c *gin.Context) {
	rc, nodeID, ok := h.nodeRequest(c)
	if !ok {
		return
	}
	revisionID, ok := canonicalUUID(c.Param("revisionId"))
	if !ok {
		invalidRequest(c)
		return
	}
	if !requireEmptyBody(c) {
		return
	}
	if h.restoreRevision == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	err := h.restoreRevision(c.Request.Context(), command.RestoreRevision{CommandID: rc.commandID, ActorID: rc.actorID, OrganizationID: rc.namespace.OrganizationID, WorkspaceID: rc.workspaceID, NamespaceID: rc.namespace.ID, NodeID: nodeID, RevisionID: revisionID, Authorization: h.resourceDecision(c, rc, nodeID, model.ResourcePermissionWrite)})
	if err != nil {
		filesystemError(c, err)
		return
	}
	httpx.OK(c, gin.H{"restored": true})
}

var errResponseWritten = errors.New("response already written")

func (h *Handler) nodeRequest(c *gin.Context) (requestContext, uuid.UUID, bool) {
	rc, ok := h.requestContext(c, true)
	if !ok {
		return requestContext{}, uuid.Nil, false
	}
	nodeID, ok := canonicalUUID(c.Param("nodeId"))
	if !ok {
		invalidRequest(c)
		return requestContext{}, uuid.Nil, false
	}
	return rc, nodeID, true
}

func (h *Handler) withNodeMutation(c *gin.Context, mutate func(requestContext, uuid.UUID) error) {
	rc, nodeID, ok := h.nodeRequest(c)
	if !ok {
		return
	}
	if err := mutate(rc, nodeID); err != nil {
		if !errors.Is(err, errResponseWritten) {
			filesystemError(c, err)
		}
		return
	}
	httpx.OK(c, gin.H{"updated": true})
}

func decodeJSON(c *gin.Context, dst any) bool {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		httpx.Fail(c, "FILESYSTEM_UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return false
	}
	if c.Request.ContentLength > maxJSONBodyBytes {
		invalidRequest(c)
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		invalidRequest(c)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		invalidRequest(c)
		return false
	}
	return true
}

func requireEmptyBody(c *gin.Context) bool {
	if c.Request.Body == nil {
		return true
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodyBytes)
	var one [1]byte
	n, err := c.Request.Body.Read(one[:])
	if n != 0 || (err != nil && !errors.Is(err, io.EOF)) {
		invalidRequest(c)
		return false
	}
	return true
}

func pagination(c *gin.Context) (int, bool) {
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 200 {
			invalidRequest(c)
			return 0, false
		}
		limit = value
	}
	if cursor := c.Query("cursor"); cursor != "" {
		if len(cursor) > 128 {
			invalidRequest(c)
			return 0, false
		}
		if _, ok := canonicalUUID(cursor); !ok {
			invalidRequest(c)
			return 0, false
		}
	}
	return limit, true
}

func optionalUUIDQuery(c *gin.Context, name string) (*uuid.UUID, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	return optionalUUID(&raw)
}

func optionalUUID(raw *string) (*uuid.UUID, bool) {
	if raw == nil {
		return nil, true
	}
	id, ok := canonicalUUID(*raw)
	if !ok {
		return nil, false
	}
	return &id, true
}

func canonicalUUID(raw string) (uuid.UUID, bool) {
	if raw == "" || strings.TrimSpace(raw) != raw || raw != strings.ToLower(raw) || len(raw) != 36 {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return uuid.Nil, false
	}
	return id, true
}

func invalidRequest(c *gin.Context) {
	httpx.Fail(c, "FILESYSTEM_INVALID_REQUEST", "invalid request", http.StatusBadRequest)
}

func filesystemError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrResourceDenied):
		httpx.Fail(c, "FILESYSTEM_RESOURCE_FORBIDDEN", "filesystem resource access denied", http.StatusForbidden)
	case errors.Is(err, domain.ErrNamespaceNotFound), errors.Is(err, domain.ErrNodeNotFound), errors.Is(err, domain.ErrRevisionNotFound):
		httpx.Fail(c, "FILESYSTEM_NOT_FOUND", "filesystem resource not found", http.StatusNotFound)
	case errors.Is(err, domain.ErrNamespaceMismatch):
		httpx.Fail(c, "FILESYSTEM_NOT_FOUND", "filesystem resource not found", http.StatusNotFound)
	case errors.Is(err, domain.ErrNodeNameConflict), errors.Is(err, domain.ErrNodeIDConflict),
		errors.Is(err, domain.ErrRevisionConflict), errors.Is(err, domain.ErrIdempotencyConflict),
		errors.Is(err, domain.ErrCommandInProgress), errors.Is(err, domain.ErrRootMutation), errors.Is(err, domain.ErrNodeStateConflict):
		httpx.Fail(c, "FILESYSTEM_CONFLICT", "filesystem operation conflict", http.StatusConflict)
	case errors.Is(err, domain.ErrQuotaExceeded):
		httpx.Fail(c, "FILESYSTEM_QUOTA_EXCEEDED", "filesystem quota exceeded", http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrTransferNotFound):
		httpx.Fail(c, "FILESYSTEM_NOT_FOUND", "filesystem resource not found", http.StatusNotFound)
	case errors.Is(err, domain.ErrTransferIdempotencyConflict), errors.Is(err, domain.ErrTransferConflict),
		errors.Is(err, domain.ErrTransferVersionConflict), errors.Is(err, domain.ErrTransferStatusConflict):
		httpx.Fail(c, "FILESYSTEM_CONFLICT", "filesystem operation conflict", http.StatusConflict)
	case errors.Is(err, domain.ErrTransferExpired):
		httpx.Fail(c, "FILESYSTEM_UNSUPPORTED_OPERATION", "filesystem operation is unsupported", http.StatusUnprocessableEntity)
	case errors.Is(err, model.ErrInvalidTransfer), errors.Is(err, model.ErrInvalidTransferSize),
		errors.Is(err, model.ErrInvalidTransferChecksum), errors.Is(err, model.ErrInvalidTransferProtocol),
		errors.Is(err, model.ErrInvalidTransferName), errors.Is(err, model.ErrInvalidTransferExpiry):
		httpx.Fail(c, "FILESYSTEM_INVALID_REQUEST", "invalid request", http.StatusBadRequest)
	case errors.Is(err, domain.ErrUnsupportedNodeType):
		httpx.Fail(c, "FILESYSTEM_UNSUPPORTED_OPERATION", "filesystem operation is unsupported", http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrParentNotFound), errors.Is(err, domain.ErrParentNotDirectory), errors.Is(err, domain.ErrDirectoryCycle), errors.Is(err, model.ErrInvalidNodeName):
		httpx.Fail(c, "FILESYSTEM_INVALID_REQUEST", "invalid filesystem request", http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrOutcomeUnknown), errors.Is(err, storage.ErrOutcomeUnknown):
		httpx.Fail(c, "FILESYSTEM_OUTCOME_UNKNOWN", "filesystem operation outcome unknown", http.StatusServiceUnavailable)
	case errors.Is(err, domain.ErrTransactionRetry), errors.Is(err, domain.ErrTransactionExhausted):
		httpx.Fail(c, "FILESYSTEM_RETRY_EXHAUSTED", "filesystem operation temporarily unavailable", http.StatusServiceUnavailable)
	case errors.Is(err, domain.ErrPersistence):
		httpx.Fail(c, "FILESYSTEM_UNAVAILABLE", "filesystem service unavailable", http.StatusServiceUnavailable)
	default:
		httpx.Fail(c, "FILESYSTEM_INTERNAL_ERROR", "filesystem operation failed", http.StatusInternalServerError)
	}
}

func created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, httpx.Envelope{Code: 0, Message: "ok", Data: data, TraceID: httpx.GetTraceID(c)})
}

func namespaceView(namespace *model.Namespace) gin.H {
	return gin.H{"id": namespace.ID, "workspace_id": namespace.WorkspaceID, "root_node_id": namespace.RootNodeID, "status": namespace.Status}
}

func nodeView(node *model.Node) gin.H {
	return gin.H{"id": node.ID, "namespace_id": node.NamespaceID, "parent_id": node.ParentID, "type": node.Type, "name": node.Name, "current_revision_id": node.CurrentRevisionID, "state": node.State, "owner_id": node.OwnerID}
}

func revisionView(revision *model.FileRevision) gin.H {
	return gin.H{"id": revision.ID(), "node_id": revision.NodeID(), "revision_no": revision.RevisionNo(), "size": revision.Size(), "mime_type": revision.MIMEType(), "created_by": revision.CreatedBy()}
}

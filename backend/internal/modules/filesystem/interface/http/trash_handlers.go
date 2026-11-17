package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type TrashHandlers struct {
	ListTrash               func(context.Context, query.ListTrash) (query.ListTrashResult, error)
	RestoreNode             func(context.Context, command.RestoreNode) error
	PermanentDeleteNode     func(context.Context, command.PermanentDeleteNode) error
	EmptyTrash              func(context.Context, command.EmptyTrash) (command.EmptyTrashResult, error)
	GetNodeIncludingTrashed func(context.Context, uuid.UUID, uuid.UUID) (*model.Node, error)
}

func (h *Handler) BindTrash(handlers TrashHandlers) {
	if h == nil {
		return
	}
	h.listTrash = handlers.ListTrash
	h.restoreNode = handlers.RestoreNode
	h.permanentDeleteNode = handlers.PermanentDeleteNode
	h.emptyTrash = handlers.EmptyTrash
	h.getNodeIncludingTrashed = handlers.GetNodeIncludingTrashed
}

func (h *Handler) ListTrash(c *gin.Context) {
	rc, ok := h.requestContext(c, false)
	if !ok {
		return
	}
	namespace, ok := h.namespaceForReadRequest(c, rc)
	if !ok {
		return
	}
	limit, ok := pagination(c)
	if !ok {
		return
	}
	if h.listTrash == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	result, err := h.listTrash(c.Request.Context(), query.ListTrash{
		NamespaceID: namespace.ID,
		Limit:       limit,
		Cursor:      c.Query("cursor"),
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	views := make([]gin.H, 0, len(result.Items))
	for i := range result.Items {
		entry := result.Items[i]
		canRestore := false
		canPurge := false
		if h.authorizeResource != nil {
			parentID := entry.Node.ParentID
			if parentID == nil {
				parentID = namespace.RootNodeID
			}
			if parentID != nil {
				restoreDecision := h.authorizeResource(c.Request.Context(), resourceauthRequest(rc, namespace.ID, *parentID, model.ResourcePermissionWrite))
				canRestore = restoreDecision.Allowed
			}
			deleteDecision := h.authorizeResource(c.Request.Context(), resourceauthRequest(rc, namespace.ID, entry.Node.ID, model.ResourcePermissionDelete))
			canPurge = deleteDecision.Allowed
		}
		views = append(views, trashItemView(&entry, canRestore, canPurge))
	}
	httpx.OK(c, gin.H{"items": views, "next_cursor": result.NextCursor})
}

func (h *Handler) RestoreNode(c *gin.Context) {
	rc, nodeID, ok := h.nodeRequest(c)
	if !ok {
		return
	}
	if !requireEmptyBody(c) {
		return
	}
	targetNS, ok := h.resolveMutationNamespace(c, rc, strings.TrimSpace(c.Query("namespace_id")))
	if !ok {
		return
	}
	rc.namespace = targetNS
	if h.restoreNode == nil || h.getNodeIncludingTrashed == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	node, err := h.getNodeIncludingTrashed(c.Request.Context(), targetNS.ID, nodeID)
	if err != nil {
		filesystemError(c, err)
		return
	}
	parentID := node.ParentID
	if parentID == nil {
		parentID = targetNS.RootNodeID
	}
	if parentID == nil {
		filesystemError(c, domain.ErrRestoreParentUnavailable)
		return
	}
	if !h.requireResourceInNamespace(c, rc, targetNS.ID, *parentID, model.ResourcePermissionWrite) {
		return
	}
	authorization := h.resourceDecisionForNamespace(c, rc, targetNS.ID, *parentID, model.ResourcePermissionWrite)
	err = h.restoreNode(c.Request.Context(), command.RestoreNode{
		CommandID: rc.commandID, ActorID: rc.actorID, OrganizationID: targetNS.OrganizationID,
		WorkspaceID: rc.workspaceID, NamespaceID: targetNS.ID, NodeID: nodeID, Authorization: authorization,
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) PermanentDeleteNode(c *gin.Context) {
	rc, nodeID, ok := h.nodeRequest(c)
	if !ok {
		return
	}
	if !requireEmptyBody(c) {
		return
	}
	targetNS, ok := h.resolveMutationNamespace(c, rc, strings.TrimSpace(c.Query("namespace_id")))
	if !ok {
		return
	}
	rc.namespace = targetNS
	if h.permanentDeleteNode == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	if !h.requireResourceInNamespace(c, rc, targetNS.ID, nodeID, model.ResourcePermissionDelete) {
		return
	}
	authorization := h.resourceDecisionForNamespace(c, rc, targetNS.ID, nodeID, model.ResourcePermissionDelete)
	err := h.permanentDeleteNode(c.Request.Context(), command.PermanentDeleteNode{
		CommandID: rc.commandID, ActorID: rc.actorID, OrganizationID: targetNS.OrganizationID,
		WorkspaceID: rc.workspaceID, NamespaceID: targetNS.ID, NodeID: nodeID, Authorization: authorization,
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) EmptyTrash(c *gin.Context) {
	rc, ok := h.requestContext(c, true)
	if !ok {
		return
	}
	if !requireEmptyBody(c) {
		return
	}
	targetNS, ok := h.resolveMutationNamespace(c, rc, strings.TrimSpace(c.Query("namespace_id")))
	if !ok {
		return
	}
	rc.namespace = targetNS
	if h.emptyTrash == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	result, err := h.emptyTrash(c.Request.Context(), command.EmptyTrash{
		CommandID: rc.commandID, ActorID: rc.actorID, OrganizationID: targetNS.OrganizationID,
		WorkspaceID: rc.workspaceID, NamespaceID: targetNS.ID,
		Authorization: command.Authorization{Allowed: true, WorkspaceID: rc.workspaceID, TraceID: rc.traceID},
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": emptyTrashItemViews(result.Items)})
}

func resourceauthRequest(rc requestContext, namespaceID, nodeID uuid.UUID, permission model.ResourcePermission) resourceauth.Request {
	return resourceauth.Request{
		ActorID: rc.actorID, WorkspaceID: rc.workspaceID, NamespaceID: namespaceID,
		NodeID: nodeID, Permission: permission,
	}
}

func trashItemView(entry *repository.TrashEntry, canRestore, canPermanentlyDelete bool) gin.H {
	view := gin.H{
		"node":                   nodeView(&entry.Node),
		"original_path":          entry.OriginalPath,
		"deleted_at":             entry.DeletedAt.UTC().Format(time.RFC3339Nano),
		"deleted_by":             entry.DeletedBy,
		"can_restore":            canRestore,
		"can_permanently_delete": canPermanentlyDelete,
	}
	return view
}

func emptyTrashItemViews(items []command.EmptyTrashItemResult) []gin.H {
	views := make([]gin.H, 0, len(items))
	for _, item := range items {
		row := gin.H{"node_id": item.NodeID, "succeeded": item.Error == nil}
		if item.Error != nil {
			row["error_code"] = batchItemErrorCode(item.Error)
		}
		views = append(views, row)
	}
	return views
}

func batchItemErrorCode(err error) string {
	switch {
	case errors.Is(err, domain.ErrRetentionHold):
		return "FILESYSTEM_RETENTION_HOLD"
	case errors.Is(err, domain.ErrResourceDenied):
		return "FILESYSTEM_RESOURCE_FORBIDDEN"
	case errors.Is(err, domain.ErrNodeStateConflict), errors.Is(err, domain.ErrIdempotencyConflict):
		return "FILESYSTEM_CONFLICT"
	case errors.Is(err, domain.ErrNodeNotFound), errors.Is(err, domain.ErrNamespaceNotFound):
		return "FILESYSTEM_NOT_FOUND"
	default:
		return "FILESYSTEM_INTERNAL_ERROR"
	}
}

package http

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type SearchHandlers struct {
	Search func(context.Context, query.SearchFiles) ([]query.SearchHit, string, error)
	GetPreviewBlob func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) (uuid.UUID, error)
}

func (h *Handler) BindSearch(handlers SearchHandlers) {
	if h == nil {
		return
	}
	h.searchFiles = handlers.Search
	h.getPreviewBlob = handlers.GetPreviewBlob
}

func (h *Handler) SearchFiles(c *gin.Context) {
	rc, ok := h.requestContext(c, false)
	if !ok {
		return
	}
	limit, ok := pagination(c)
	if !ok {
		return
	}
	var ownerID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("owner_id")); raw != "" {
		id, valid := canonicalUUID(raw)
		if !valid {
			invalidRequest(c)
			return
		}
		ownerID = &id
	}
	if h.searchFiles == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	hits, nextCursor, err := h.searchFiles(c.Request.Context(), query.SearchFiles{
		ActorID: rc.actorID, WorkspaceID: rc.workspaceID, NamespaceID: rc.namespace.ID,
		Query: strings.TrimSpace(c.Query("q")), OwnerID: ownerID, Limit: limit, Cursor: c.Query("cursor"),
		FullText: strings.EqualFold(c.Query("fulltext"), "true"),
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	items := make([]gin.H, 0, len(hits))
	for _, hit := range hits {
		node, nodeErr := h.getNode(c.Request.Context(), rc.namespace.ID, hit.NodeID)
		if nodeErr != nil {
			continue
		}
		items = append(items, nodeView(node))
	}
	payload := gin.H{"items": items}
	if nextCursor != "" {
		payload["next_cursor"] = nextCursor
	}
	httpx.OK(c, payload)
}

func (h *Handler) GetFilePreview(c *gin.Context) {
	rc, ok := h.requestContext(c, false)
	if !ok {
		return
	}
	nodeID, ok := canonicalUUID(c.Param("nodeId"))
	if !ok {
		invalidRequest(c)
		return
	}
	if !h.requireResource(c, rc, nodeID, model.ResourcePermissionRead) {
		return
	}
	node, err := h.getNode(c.Request.Context(), rc.namespace.ID, nodeID)
	if err != nil {
		filesystemError(c, err)
		return
	}
	revisionID := node.CurrentRevisionID
	if raw := strings.TrimSpace(c.Query("revision_id")); raw != "" {
		id, valid := canonicalUUID(raw)
		if !valid {
			invalidRequest(c)
			return
		}
		revisionID = &id
	}
	if revisionID == nil {
		filesystemError(c, domain.ErrRevisionNotFound)
		return
	}
	if h.getPreviewBlob == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	blobID, err := h.getPreviewBlob(c.Request.Context(), rc.namespace.ID, nodeID, revisionID)
	if err != nil {
		filesystemError(c, err)
		return
	}
	httpx.OK(c, gin.H{"node_id": nodeID, "revision_id": *revisionID, "preview_blob_id": blobID})
}

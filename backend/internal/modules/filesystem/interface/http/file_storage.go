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

func (h *Handler) BindFileStorage(handler func(context.Context, query.GetFileStorage) (query.FileStorageView, error)) {
	if h == nil {
		return
	}
	h.getFileStorage = handler
}

func (h *Handler) GetFileStorage(c *gin.Context) {
	rc, ok := h.requestContext(c, false)
	if !ok {
		return
	}
	namespace, ok := h.namespaceForReadRequest(c, rc)
	if !ok {
		return
	}
	nodeID, ok := canonicalUUID(c.Param("nodeId"))
	if !ok {
		invalidRequest(c)
		return
	}
	if !h.requireResourceInNamespace(c, rc, namespace.ID, nodeID, model.ResourcePermissionRead) {
		return
	}
	var revisionID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("revision_id")); raw != "" {
		id, valid := canonicalUUID(raw)
		if !valid {
			invalidRequest(c)
			return
		}
		revisionID = &id
	}
	if h.getFileStorage == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	view, err := h.getFileStorage(c.Request.Context(), query.GetFileStorage{
		NamespaceID: namespace.ID,
		NodeID:      nodeID,
		RevisionID:  revisionID,
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	payload := gin.H{
		"revision_id":      view.RevisionID,
		"blob_id":          view.BlobID,
		"provider_id":      view.ProviderID,
		"provider_type":    view.ProviderType,
		"provider_root":    view.ProviderRoot,
		"object_key":       view.ObjectKey,
		"content_sha256":   view.ContentSHA256,
	}
	if view.LocalObjectFile != "" {
		payload["local_object_file"] = view.LocalObjectFile
	}
	httpx.OK(c, payload)
}
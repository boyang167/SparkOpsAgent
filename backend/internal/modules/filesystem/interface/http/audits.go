package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

func (h *Handler) ListFilesystemAudits(c *gin.Context) {
	rc, ok := h.requestContext(c, false)
	if !ok {
		return
	}
	if h.listFileAudits == nil {
		httpx.Fail(c, "NOT_IMPLEMENTED", "filesystem audit listing unavailable", http.StatusNotImplemented)
		return
	}
	request, ok := fileAuditListRequest(c, rc.namespace.OrganizationID, rc.workspaceID)
	if !ok {
		return
	}
	h.writeFileAuditList(c, request)
}

func fileAuditListRequest(c *gin.Context, organizationID, workspaceID uuid.UUID) (query.ListFileAuditEvents, bool) {
	var actorID *uuid.UUID
	if raw := c.Query("actor_id"); raw != "" {
		id, valid := canonicalUUID(raw)
		if !valid {
			invalidRequest(c)
			return query.ListFileAuditEvents{}, false
		}
		actorID = &id
	}
	metadataIDs := map[string]string{}
	for _, key := range []string{"namespace_id", "node_id", "principal_id"} {
		if raw := c.Query(key); raw != "" {
			id, valid := canonicalUUID(raw)
			if !valid {
				invalidRequest(c)
				return query.ListFileAuditEvents{}, false
			}
			metadataIDs[key] = id.String()
		}
	}
	var from, to *time.Time
	if raw := c.Query("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			invalidRequest(c)
			return query.ListFileAuditEvents{}, false
		}
		from = &parsed
	}
	if raw := c.Query("to"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			invalidRequest(c)
			return query.ListFileAuditEvents{}, false
		}
		to = &parsed
	}
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 100 {
			invalidRequest(c)
			return query.ListFileAuditEvents{}, false
		}
		limit = parsed
	}
	return query.ListFileAuditEvents{
		OrganizationID: organizationID,
		WorkspaceID:    workspaceID,
		ActorID:        actorID,
		Action:         c.Query("action"),
		ResourceID:     c.Query("resource_id"),
		NamespaceID:    metadataIDs["namespace_id"],
		NodeID:         metadataIDs["node_id"],
		PrincipalID:    metadataIDs["principal_id"],
		Result:         c.Query("result"),
		From:           from,
		To:             to,
		Cursor:         c.Query("cursor"),
		Limit:          limit,
	}, true
}

func (h *Handler) writeFileAuditList(c *gin.Context, request query.ListFileAuditEvents) {
	result, err := h.listFileAudits(c.Request.Context(), request)
	if err != nil {
		filesystemError(c, err)
		return
	}
	items := make([]gin.H, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, gin.H{
			"id": item.ID, "actor_id": item.ActorID, "action": item.Action,
			"resource_type": item.ResourceType, "resource_id": item.ResourceID,
			"result": item.Result, "trace_id": item.TraceID, "created_at": item.CreatedAt,
			"detail": item.Detail,
		})
	}
	httpx.OK(c, gin.H{"items": items, "next_cursor": result.NextCursor})
}

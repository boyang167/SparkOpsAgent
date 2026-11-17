package http

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func (h *Handler) GetFileContentAccess(c *gin.Context) {
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
	if h.getNode == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	node, err := h.getNode(c.Request.Context(), namespace.ID, nodeID)
	if err != nil {
		filesystemError(c, err)
		return
	}
	if node.Type != model.NodeTypeFile {
		filesystemError(c, domain.ErrUnsupportedNodeType)
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
	if h.getRevision == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	revision, err := h.getRevision(c.Request.Context(), nodeID, *revisionID)
	if err != nil {
		filesystemError(c, err)
		return
	}
	previewKind := model.ClassifyFilePreview(revision.MIMEType(), node.Name)
	maxBytes := model.MaxPreviewBytes(previewKind)
	payload := gin.H{
		"revision_id":       *revisionID,
		"mime_type":         revision.MIMEType(),
		"size":              revision.Size(),
		"file_name":         node.Name,
		"preview_kind":      string(previewKind),
		"preview_supported": false,
	}
	if previewKind == model.FilePreviewKindUnsupported {
		payload["reason_code"] = "PREVIEW_UNSUPPORTED"
		httpx.OK(c, payload)
		return
	}
	if revision.Size() > maxBytes {
		payload["reason_code"] = "PREVIEW_TOO_LARGE"
		payload["max_preview_bytes"] = maxBytes
		httpx.OK(c, payload)
		return
	}
	if h.audit != nil && (node.Classification == model.TransferClassificationConfidential ||
		node.Classification == model.TransferClassificationRestricted) {
		if err := h.audit.Log(c.Request.Context(), audit.Entry{
			ActorID: kernel.MemberID{UUID: rc.actorID}, OrgID: &kernel.OrgID{UUID: rc.namespace.OrganizationID},
			Action: "filesystem.file.content_access", ResourceType: "filesystem", ResourceID: node.ID.String(),
			Detail: map[string]any{
				"workspace_id": rc.workspaceID.String(), "namespace_id": namespace.ID.String(),
				"node_id": node.ID.String(), "revision_id": revisionID.String(),
				"principal_id": rc.actorID.String(), "preview_kind": string(previewKind),
				"trace_id": rc.traceID, "classification": node.Classification,
			},
		}); err != nil {
			filesystemError(c, err)
			return
		}
	}
	gw := h.transferGatewayView
	base := strings.TrimRight(strings.TrimSpace(gw.PublicURL), "/")
	if base == "" || gw.Issuer == nil {
		payload["reason_code"] = "PREVIEW_GATEWAY_UNAVAILABLE"
		httpx.OK(c, payload)
		return
	}
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	token, err := gw.Issuer.DownloadToken(nodeID, expiresAt)
	if err != nil {
		filesystemError(c, err)
		return
	}
	payload["preview_supported"] = true
	payload["download_url"] = base + "/v1/files/" + nodeID.String() + "/content"
	payload["capability_token"] = token
	payload["expires_at"] = expiresAt.Format(time.RFC3339)
	httpx.OK(c, payload)
}
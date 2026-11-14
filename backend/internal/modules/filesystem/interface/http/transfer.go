package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/capability"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type TransferGatewayView struct {
	PublicURL string
	Issuer    *capability.Issuer
}

func NewTransferGatewayView(publicURL string, issuer *capability.Issuer) TransferGatewayView {
	return TransferGatewayView{PublicURL: publicURL, Issuer: issuer}
}

type createTransferRequest struct {
	NamespaceID    string                       `json:"namespace_id"`
	ParentID       *string                      `json:"parent_id"`
	Name           string                       `json:"name"`
	Protocol       model.TransferProtocol       `json:"protocol"`
	Size           int64                        `json:"size"`
	SHA256         string                       `json:"sha256"`
	Classification model.TransferClassification `json:"classification"`
	ProviderID     string                       `json:"provider_id"`
	ExpiresAt      *time.Time                   `json:"expires_at"`
}

func (h *Handler) CreateTransfer(c *gin.Context) {
	rc, ok := h.requestContext(c, true)
	if !ok {
		return
	}
	var body createTransferRequest
	if !decodeJSON(c, &body) {
		return
	}
	parentID, ok := optionalUUID(body.ParentID)
	if !ok || strings.TrimSpace(body.Name) == "" {
		invalidRequest(c)
		return
	}
	var providerID uuid.UUID
	if strings.TrimSpace(body.ProviderID) != "" {
		var ok bool
		providerID, ok = canonicalUUID(body.ProviderID)
		if !ok {
			invalidRequest(c)
			return
		}
	}
	if body.Protocol != model.TransferProtocolDirect && body.Protocol != model.TransferProtocolMultipart {
		invalidRequest(c)
		return
	}
	if body.Classification == "" {
		body.Classification = model.TransferClassificationInternal
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	if body.ExpiresAt != nil {
		expiresAt = model.CanonicalTransferExpiry(*body.ExpiresAt)
	}
	if h.createTransfer == nil {
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
	cmd := command.CreateTransfer{
		ActorID: rc.actorID, OrganizationID: rc.namespace.OrganizationID,
		WorkspaceID: rc.workspaceID, NamespaceID: rc.namespace.ID, ParentID: parentID,
		Name: body.Name, Protocol: body.Protocol, Size: body.Size,
		SHA256: strings.ToLower(strings.TrimSpace(body.SHA256)), Classification: body.Classification,
		IdempotencyKey: c.GetString(idempotencyContextKey), ExpiresAt: expiresAt, ProviderID: providerID,
	}
	if body.NamespaceID != "" {
		namespaceID, valid := canonicalUUID(body.NamespaceID)
		if !valid || namespaceID != rc.namespace.ID {
			filesystemError(c, domain.ErrNamespaceMismatch)
			return
		}
	}
	auth := h.resourceDecision(c, rc, *parentID, model.ResourcePermissionWrite)
	auth.TraceID = rc.traceID
	cmd.Authorization = auth
	descriptor, err := h.createTransfer(c.Request.Context(), cmd)
	if err != nil {
		filesystemError(c, err)
		return
	}
	sessionID := descriptor.SessionID
	result, err := h.getTransferAuthorized(c, rc, sessionID, model.ResourcePermissionRead)
	if err != nil {
		filesystemError(c, err)
		return
	}
	created(c, transferSessionView(result, h.transferGatewayView))
}

func (h *Handler) GetTransfer(c *gin.Context) {
	rc, ok := h.requestContext(c, false)
	if !ok {
		return
	}
	sessionID, ok := canonicalUUID(c.Param("sessionId"))
	if !ok {
		invalidRequest(c)
		return
	}
	if h.getTransfer == nil || h.peekTransfer == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	result, err := h.getTransferAuthorized(c, rc, sessionID, model.ResourcePermissionRead)
	if err != nil {
		filesystemError(c, err)
		return
	}
	httpx.OK(c, transferSessionView(result, h.transferGatewayView))
}

func (h *Handler) ListTransfers(c *gin.Context) {
	rc, ok := h.requestContext(c, false)
	if !ok {
		return
	}
	limit, ok := transferListPagination(c)
	if !ok {
		return
	}
	status := model.TransferStatus(c.Query("status"))
	if status != "" && !validTransferStatusQuery(status) {
		invalidRequest(c)
		return
	}
	if h.listTransfers == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	items, next, err := h.listTransfers(c.Request.Context(), query.ListTransfers{
		WorkspaceID: rc.workspaceID,
		Status:      status,
		Limit:       limit,
		Cursor:      c.Query("cursor"),
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	views := make([]gin.H, 0, len(items))
	for _, session := range items {
		views = append(views, transferSessionView(query.GetTransferResult{
			Session:    session,
			Descriptor: command.DescriptorFromSession(session),
		}, TransferGatewayView{}))
	}
	httpx.OK(c, gin.H{"items": views, "next_cursor": next})
}

func transferListPagination(c *gin.Context) (int, bool) {
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			invalidRequest(c)
			return 0, false
		}
		limit = value
	}
	if cursor := c.Query("cursor"); len(cursor) > 256 {
		invalidRequest(c)
		return 0, false
	}
	return limit, true
}

func validTransferStatusQuery(status model.TransferStatus) bool {
	switch status {
	case model.TransferInitiated, model.TransferUploading, model.TransferPaused,
		model.TransferVerifying, model.TransferScanning, model.TransferCommitting,
		model.TransferCompleted, model.TransferFailed, model.TransferRejected,
		model.TransferAborted, model.TransferExpired:
		return true
	default:
		return false
	}
}

func (h *Handler) AbortTransfer(c *gin.Context) {
	rc, ok := h.requestContext(c, false)
	if !ok {
		return
	}
	sessionID, ok := canonicalUUID(c.Param("sessionId"))
	if !ok {
		invalidRequest(c)
		return
	}
	if !requireEmptyBody(c) {
		return
	}
	if h.abortTransfer == nil || h.peekTransfer == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	parent, err := h.transferParent(c.Request.Context(), rc, sessionID)
	if err != nil {
		filesystemError(c, err)
		return
	}
	auth := h.resourceDecision(c, rc, parent, model.ResourcePermissionWrite)
	err = h.abortTransfer(c.Request.Context(), command.AbortTransfer{
		ActorID: rc.actorID, WorkspaceID: rc.workspaceID, SessionID: sessionID,
		Authorization: auth,
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) getTransferAuthorized(c *gin.Context, rc requestContext, sessionID uuid.UUID, permission model.ResourcePermission) (query.GetTransferResult, error) {
	var out query.GetTransferResult
	parent, err := h.transferParent(c.Request.Context(), rc, sessionID)
	if err != nil {
		return out, err
	}
	return h.getTransfer(c.Request.Context(), query.GetTransfer{
		WorkspaceID: rc.workspaceID, SessionID: sessionID,
		Authorization: h.resourceDecision(c, rc, parent, permission),
	})
}

func (h *Handler) transferParent(ctx context.Context, rc requestContext, sessionID uuid.UUID) (uuid.UUID, error) {
	session, err := h.peekTransfer(ctx, sessionID)
	if err != nil {
		return uuid.Nil, err
	}
	if session.WorkspaceID() != rc.workspaceID {
		return uuid.Nil, domain.ErrTransferNotFound
	}
	parent := session.TargetParentID()
	if parent == nil {
		parent = rc.namespace.RootNodeID
	}
	if parent == nil {
		return uuid.Nil, domain.ErrParentNotFound
	}
	return *parent, nil
}

func transferSessionView(result query.GetTransferResult, gw TransferGatewayView) gin.H {
	view := gin.H{
		"session_id":       result.Session.ID(),
		"workspace_id":     result.Session.WorkspaceID(),
		"namespace_id":     result.Session.NamespaceID(),
		"target_parent_id": result.Session.TargetParentID(),
		"target_name":      result.Session.TargetName(),
		"status":           result.Session.Status(),
		"protocol":         result.Session.Protocol(),
		"expected_size":    result.Session.ExpectedSize(),
		"expected_sha256":  result.Session.ExpectedSHA256(),
		"part_size":        result.Descriptor.PartSize,
		"total_parts":      result.Descriptor.TotalParts,
		"expires_at":       result.Session.ExpiresAt().UTC().Format(time.RFC3339Nano),
		"error_code":       result.Session.ErrorCode(),
		"parts":            transferPartsView(result.Parts),
	}
	if base := strings.TrimRight(strings.TrimSpace(gw.PublicURL), "/"); base != "" && gw.Issuer != nil &&
		result.Session.Protocol() == model.TransferProtocolDirect && result.Session.ExpectedSize() >= 0 {
		view["upload_url"] = base + "/v1/transfers/" + result.Session.ID().String() + "/content"
		if token, err := gw.Issuer.UploadToken(result.Session.ID(), result.Session.ExpiresAt()); err == nil {
			view["capability_token"] = token
		}
	}
	return view
}

func transferPartsView(parts []model.TransferPart) []gin.H {
	views := make([]gin.H, 0, len(parts))
	for i := range parts {
		views = append(views, gin.H{
			"part_number": parts[i].PartNumber(),
			"offset":      parts[i].Offset(),
			"size":        parts[i].Size(),
			"etag":        parts[i].ETag(),
			"checksum":    parts[i].Checksum(),
		})
	}
	return views
}

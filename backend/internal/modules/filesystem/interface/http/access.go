package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/operation"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	domainservice "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ResourceAccess struct {
	Grant            func(ctx context.Context, cmd command.GrantResource) (model.ResourceGrant, error)
	Share            func(ctx context.Context, cmd command.SubmitShare) (command.ShareResult, error)
	ListGrants       func(ctx context.Context, cmd command.GrantResource) ([]model.ResourceGrant, error)
	RevokeGrant      func(ctx context.Context, cmd command.GrantResource, grantID uuid.UUID) error
	IssueCapability  func(ctx context.Context, cmd command.IssueCapability) (command.IssuedCapability, error)
	RevokeCapability func(ctx context.Context, cmd command.RevokeCapability) error
	StartExport      func(ctx context.Context, cmd command.StartControlledExport) (model.ExportTask, error)
	ImportExported   func(ctx context.Context, cmd operation.ImportExportedFile) (operation.ImportResult, error)
}

func (h *Handler) BindResourceAccess(access ResourceAccess) {
	if h == nil {
		return
	}
	h.access = access
}

type grantBody struct {
	GranteeID             string     `json:"grantee_id"`
	GranteeOrganizationID string     `json:"grantee_organization_id"`
	Permission            string     `json:"permission"`
	Effect                string     `json:"effect"`
	PrincipalType         string     `json:"principal_type"`
	PrincipalIDs          []string   `json:"principal_ids"`
	Actions               []string   `json:"actions"`
	InheritToChildren     *bool      `json:"inherit_to_children"`
	ExpiresAt             *time.Time `json:"expires_at"`
	Anonymous             bool       `json:"anonymous"`
}

func (h *Handler) CreateGrant(c *gin.Context) {
	rc, nodeID, ok := h.nodeRequest(c)
	if !ok {
		return
	}
	var body grantBody
	if !decodeJSON(c, &body) {
		return
	}
	if h.access.Share != nil {
		submit, legacy, valid := shareCommand(c, rc, nodeID, body)
		if !valid {
			return
		}
		result, err := h.access.Share(c.Request.Context(), submit)
		if err != nil {
			filesystemError(c, err)
			return
		}
		if result.Request.Status == model.ShareRequestPending {
			c.JSON(http.StatusAccepted, gin.H{"data": shareRequestView(result.Request)})
			return
		}
		if legacy && len(result.Grants) == 1 {
			created(c, grantView(result.Grants[0]))
			return
		}
		created(c, shareResultView(result))
		return
	}
	cmd, ok := grantCommand(c, rc, nodeID, body)
	if !ok {
		return
	}
	if h.access.Grant == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	grant, err := h.access.Grant(c.Request.Context(), cmd)
	if err != nil {
		filesystemError(c, err)
		return
	}
	created(c, grantView(grant))
}

func (h *Handler) ListGrants(c *gin.Context) {
	rc, ok := h.requestContext(c, false)
	if !ok {
		return
	}
	nodeID, ok := canonicalUUID(c.Param("nodeId"))
	if !ok {
		invalidRequest(c)
		return
	}
	if h.access.ListGrants == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	grants, err := h.access.ListGrants(c.Request.Context(), command.GrantResource{
		ActorID: rc.actorID, OrganizationID: rc.namespace.OrganizationID, WorkspaceID: rc.workspaceID,
		NamespaceID: rc.namespace.ID, NodeID: nodeID, TraceID: rc.traceID,
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	views := make([]gin.H, 0, len(grants))
	for _, grant := range grants {
		views = append(views, grantView(grant))
	}
	httpx.OK(c, gin.H{"items": views})
}

func (h *Handler) RevokeGrant(c *gin.Context) {
	rc, nodeID, ok := h.nodeRequest(c)
	if !ok {
		return
	}
	grantID, ok := canonicalUUID(c.Param("grantId"))
	if !ok {
		invalidRequest(c)
		return
	}
	if h.access.RevokeGrant == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	if err := h.access.RevokeGrant(c.Request.Context(), command.GrantResource{
		CommandID: rc.commandID, ActorID: rc.actorID, OrganizationID: rc.namespace.OrganizationID,
		WorkspaceID: rc.workspaceID, NamespaceID: rc.namespace.ID, NodeID: nodeID, TraceID: rc.traceID,
	}, grantID); err != nil {
		filesystemError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type issueBody struct {
	AgentID        *string   `json:"agent_id"`
	RunID          string    `json:"run_id"`
	ResourceIDs    []string  `json:"resource_ids"`
	RunResourceIDs []string  `json:"run_resource_ids"`
	Actions        []string  `json:"actions"`
	ExpiresAt      time.Time `json:"expires_at"`
}

func (h *Handler) IssueCapability(c *gin.Context) {
	rc, ok := h.requestContext(c, true)
	if !ok {
		return
	}
	var body issueBody
	if !decodeJSON(c, &body) {
		return
	}
	runID, ok := canonicalUUID(body.RunID)
	if !ok || len(body.ResourceIDs) == 0 || len(body.RunResourceIDs) == 0 || len(body.Actions) == 0 {
		invalidRequest(c)
		return
	}
	resourceIDs, ok := parseUUIDs(body.ResourceIDs)
	if !ok {
		invalidRequest(c)
		return
	}
	runResourceIDs, ok := parseUUIDs(body.RunResourceIDs)
	if !ok {
		invalidRequest(c)
		return
	}
	permissions, ok := actionPermissions(body.Actions)
	if !ok {
		invalidRequest(c)
		return
	}
	var agentID *uuid.UUID
	if body.AgentID != nil {
		parsed, valid := canonicalUUID(*body.AgentID)
		if !valid {
			invalidRequest(c)
			return
		}
		agentID = &parsed
	}
	delegator := make([]uuid.UUID, 0, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		if h.resourceAllowed(c, rc, resourceID, permissions) {
			delegator = append(delegator, resourceID)
		}
	}
	if h.access.IssueCapability == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	issued, err := h.access.IssueCapability(c.Request.Context(), command.IssueCapability{
		CommandID: rc.commandID, OrganizationID: rc.namespace.OrganizationID, WorkspaceID: rc.workspaceID,
		ActorID: rc.actorID, AgentID: agentID, RunID: runID, ResourceIDs: resourceIDs, DelegatorResourceIDs: delegator,
		RunResourceIDs: runResourceIDs, Actions: body.Actions, RunActions: body.Actions,
		Audience: domainservice.AgentCapabilityAudience, ExpiresAt: body.ExpiresAt,
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	h.auditCapability(c, rc, issued)
	view := gin.H{"id": issued.ID, "resource_ids": issued.ResourceIDs, "expires_at": issued.ExpiresAt, "replayed": issued.Replayed, "token_hash": issued.Hash}
	if issued.Token != "" {
		view["token"] = issued.Token
	}
	if issued.Replayed {
		httpx.OK(c, view)
		return
	}
	created(c, view)
}

func (h *Handler) RevokeCapability(c *gin.Context) {
	if _, ok := h.requestContext(c, true); !ok {
		return
	}
	capabilityID, ok := canonicalUUID(c.Param("capabilityId"))
	if !ok {
		invalidRequest(c)
		return
	}
	if !requireEmptyBody(c) {
		return
	}
	if h.access.RevokeCapability == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	if err := h.access.RevokeCapability(c.Request.Context(), command.RevokeCapability{ID: capabilityID}); err != nil {
		filesystemError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type exportBody struct {
	TargetOrganizationID string `json:"target_organization_id"`
	TargetWorkspaceID    string `json:"target_workspace_id"`
	SourceNodeID         string `json:"source_node_id"`
}

func (h *Handler) StartControlledExport(c *gin.Context) {
	rc, ok := h.requestContext(c, true)
	if !ok {
		return
	}
	var body exportBody
	if !decodeJSON(c, &body) {
		return
	}
	targetOrg, ok := canonicalUUID(body.TargetOrganizationID)
	if !ok {
		invalidRequest(c)
		return
	}
	targetWorkspace, ok := canonicalUUID(body.TargetWorkspaceID)
	if !ok {
		invalidRequest(c)
		return
	}
	sourceNode, ok := canonicalUUID(body.SourceNodeID)
	if !ok {
		invalidRequest(c)
		return
	}
	if h.getNode == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	if _, err := h.getNode(c.Request.Context(), rc.namespace.ID, sourceNode); err != nil {
		filesystemError(c, err)
		return
	}
	if !h.requireResource(c, rc, sourceNode, model.ResourcePermissionAdmin) {
		return
	}
	if h.access.StartExport == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	task, err := h.access.StartExport(c.Request.Context(), command.StartControlledExport{
		ID: rc.commandID, ActorID: rc.actorID, SourceOrganizationID: rc.namespace.OrganizationID,
		TargetOrganizationID: targetOrg, SourceWorkspaceID: rc.workspaceID, TargetWorkspaceID: targetWorkspace,
		SourceNodeID: sourceNode,
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	created(c, exportView(task))
}

type importBody struct {
	ParentID string `json:"parent_id"`
	Name     string `json:"name"`
}

func (h *Handler) ImportExportedFile(c *gin.Context) {
	rc, ok := h.requestContext(c, true)
	if !ok {
		return
	}
	exportID, ok := canonicalUUID(c.Param("exportId"))
	if !ok {
		invalidRequest(c)
		return
	}
	var body importBody
	if !decodeJSON(c, &body) {
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		invalidRequest(c)
		return
	}
	parentID, ok := canonicalUUID(body.ParentID)
	if !ok {
		invalidRequest(c)
		return
	}
	if !h.requireResource(c, rc, parentID, model.ResourcePermissionWrite) {
		return
	}
	if h.access.ImportExported == nil {
		filesystemError(c, domain.ErrPersistence)
		return
	}
	result, err := h.access.ImportExported(c.Request.Context(), operation.ImportExportedFile{
		ExportID: exportID, TargetOrganizationID: rc.namespace.OrganizationID, TargetNamespaceID: rc.namespace.ID,
		TargetParentID: parentID, OwnerID: rc.actorID, Name: body.Name,
	})
	if err != nil {
		filesystemError(c, err)
		return
	}
	created(c, gin.H{"node_id": result.NodeID, "source_node_id": result.SourceNodeID})
}

func grantCommand(c *gin.Context, rc requestContext, nodeID uuid.UUID, body grantBody) (command.GrantResource, bool) {
	granteeID, ok := canonicalUUID(body.GranteeID)
	if !ok {
		invalidRequest(c)
		return command.GrantResource{}, false
	}
	granteeOrg := rc.namespace.OrganizationID
	permission, ok := resourcePermission(body.Permission)
	if !ok {
		invalidRequest(c)
		return command.GrantResource{}, false
	}
	effect, ok := grantEffect(body.Effect)
	if !ok {
		invalidRequest(c)
		return command.GrantResource{}, false
	}
	return command.GrantResource{
		CommandID: rc.commandID, ActorID: rc.actorID, OrganizationID: rc.namespace.OrganizationID,
		GranteeOrganizationID: granteeOrg, WorkspaceID: rc.workspaceID, NamespaceID: rc.namespace.ID,
		NodeID: nodeID, GranteeID: granteeID, Permission: permission, Effect: effect, ExpiresAt: body.ExpiresAt,
		TraceID: rc.traceID, Anonymous: body.Anonymous,
	}, true
}

func shareCommand(c *gin.Context, rc requestContext, nodeID uuid.UUID, body grantBody) (command.SubmitShare, bool, bool) {
	inherit := true
	if body.InheritToChildren != nil {
		inherit = *body.InheritToChildren
	}
	principalType := model.PrincipalType(strings.ToUpper(strings.TrimSpace(body.PrincipalType)))
	principalIDs, ok := parseUUIDs(body.PrincipalIDs)
	actions := make([]model.ResourcePermission, 0, len(body.Actions))
	legacy := principalType == "" && len(body.PrincipalIDs) == 0 && len(body.Actions) == 0
	if legacy {
		principalType = model.PrincipalUser
		principalID, valid := canonicalUUID(body.GranteeID)
		permission, permissionValid := resourcePermission(strings.ToUpper(strings.TrimSpace(body.Permission)))
		effect := model.GrantEffect(strings.ToUpper(strings.TrimSpace(body.Effect)))
		if !valid || !permissionValid || (effect != model.GrantEffectAllow && effect != model.GrantEffectDeny) || body.Anonymous {
			invalidRequest(c)
			return command.SubmitShare{}, true, false
		}
		principalIDs = []uuid.UUID{principalID}
		actions = []model.ResourcePermission{permission}
		body.Effect = string(effect)
	} else {
		if !ok || len(principalIDs) == 0 || len(body.Actions) == 0 {
			invalidRequest(c)
			return command.SubmitShare{}, false, false
		}
		for _, raw := range body.Actions {
			action, valid := resourcePermission(strings.ToUpper(strings.TrimSpace(raw)))
			if !valid {
				invalidRequest(c)
				return command.SubmitShare{}, false, false
			}
			actions = append(actions, action)
		}
	}
	return command.SubmitShare{
		CommandID: rc.commandID, ActorID: rc.actorID, OrganizationID: rc.namespace.OrganizationID,
		WorkspaceID: rc.workspaceID, NodeID: nodeID, PrincipalType: principalType, PrincipalIDs: principalIDs,
		Actions: actions, Effect: model.GrantEffect(strings.ToUpper(strings.TrimSpace(body.Effect))),
		InheritToChildren: inherit, ExpiresAt: body.ExpiresAt,
	}, legacy, true
}

func (h *Handler) resourceAllowed(c *gin.Context, rc requestContext, resourceID uuid.UUID, permissions []model.ResourcePermission) bool {
	for _, permission := range permissions {
		decision := h.resourceDecision(c, rc, resourceID, permission)
		if !decision.Allowed {
			h.auditResourceDeny(c, rc, resourceID, decision.Reason)
			return false
		}
	}
	return true
}

func (h *Handler) auditCapability(c *gin.Context, rc requestContext, issued command.IssuedCapability) {
	if h.audit == nil || issued.Replayed {
		return
	}
	_ = h.audit.Log(c.Request.Context(), audit.Entry{
		ActorID: kernel.MemberID{UUID: rc.actorID}, OrgID: &kernel.OrgID{UUID: rc.namespace.OrganizationID},
		Action: "filesystem.capability.issued", ResourceType: "filesystem", ResourceID: issued.ID.String(),
		Detail: map[string]any{
			"workspace_id": rc.workspaceID.String(), "decision": "ALLOW",
			"token_hash": issued.Hash, "trace_id": rc.traceID,
		},
	})
}

func grantView(grant model.ResourceGrant) gin.H {
	granteeID := grant.GranteeID
	if granteeID == uuid.Nil {
		granteeID = grant.PrincipalID
	}
	return gin.H{
		"id": grant.ID, "node_id": grant.NodeID, "grantee_id": granteeID,
		"principal_type": grant.PrincipalType, "principal_id": grant.PrincipalID,
		"permission": grant.Permission, "effect": grant.Effect, "expires_at": grant.ExpiresAt,
	}
}

func shareResultView(result command.ShareResult) gin.H {
	grants := make([]gin.H, 0, len(result.Grants))
	for _, grant := range result.Grants {
		grants = append(grants, grantView(grant))
	}
	return gin.H{"request": shareRequestView(result.Request), "grants": grants}
}

func shareRequestView(request model.ShareRequest) gin.H {
	return gin.H{
		"id": request.ID, "organization_id": request.OrganizationID, "namespace_id": request.NamespaceID,
		"node_id": request.NodeID, "requested_by": request.RequestedBy, "principal_type": request.PrincipalType,
		"principal_ids": request.PrincipalIDs, "actions": request.Actions, "inherit_to_children": request.InheritToChildren,
		"expires_at": request.ExpiresAt, "classification": request.Classification, "effect": request.Effect, "status": request.Status,
		"reviewed_by": request.ReviewedBy, "reviewed_at": request.ReviewedAt, "rejection_reason": request.RejectionReason,
		"created_at": request.CreatedAt,
	}
}

func exportView(task model.ExportTask) gin.H {
	return gin.H{
		"id": task.ID, "source_organization_id": task.SourceOrganizationID, "target_organization_id": task.TargetOrganizationID,
		"source_workspace_id": task.SourceWorkspaceID, "target_workspace_id": task.TargetWorkspaceID,
		"source_node_id": task.SourceNodeID, "target_node_id": task.TargetNodeID,
		"classification": task.Classification, "status": task.Status,
	}
}

func resourcePermission(raw string) (model.ResourcePermission, bool) {
	switch model.ResourcePermission(raw) {
	case model.ResourcePermissionRead, model.ResourcePermissionDownload, model.ResourcePermissionWrite, model.ResourcePermissionDelete, model.ResourcePermissionShare, model.ResourcePermissionAdmin:
		return model.ResourcePermission(raw), true
	default:
		return "", false
	}
}

func grantEffect(raw string) (model.GrantEffect, bool) {
	switch model.GrantEffect(raw) {
	case model.GrantEffectAllow, model.GrantEffectDeny:
		return model.GrantEffect(raw), true
	default:
		return "", false
	}
}

func actionPermissions(actions []string) ([]model.ResourcePermission, bool) {
	out := make([]model.ResourcePermission, 0, len(actions))
	for _, action := range actions {
		switch action {
		case "file:read":
			out = append(out, model.ResourcePermissionRead)
		case "file:download":
			out = append(out, model.ResourcePermissionDownload)
		case "file:write":
			out = append(out, model.ResourcePermissionWrite)
		case "file:delete":
			out = append(out, model.ResourcePermissionDelete)
		case "file:share":
			out = append(out, model.ResourcePermissionShare)
		case "file:admin":
			out = append(out, model.ResourcePermissionAdmin)
		default:
			return nil, false
		}
	}
	return out, true
}

func parseUUIDs(raw []string) ([]uuid.UUID, bool) {
	out := make([]uuid.UUID, 0, len(raw))
	for _, item := range raw {
		id, ok := canonicalUUID(item)
		if !ok {
			return nil, false
		}
		out = append(out, id)
	}
	return out, true
}

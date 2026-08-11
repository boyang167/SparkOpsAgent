package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	createOrg        *command.CreateOrganizationHandler
	createWs         *command.CreateWorkspaceHandler
	listWs           *query.ListWorkspacesHandler
	listOrgs         *query.ListOrganizationsHandler
	getOrg           *query.GetOrganizationHandler
	listOrgMembers   *query.ListOrgMembersHandler
	myOrgPerms       *query.MyOrgPermissionsHandler
	inviteOrgMember  *command.InviteOrgMemberHandler
	updateOrgRole    *command.UpdateOrgMemberRoleHandler
	removeOrgMember  *command.RemoveOrgMemberHandler
	listPlatformOrgs *query.ListPlatformOrganizationsHandler
	createPlatformOrg *command.CreatePlatformOrganizationHandler
	updateOrgStatus  *command.UpdateOrganizationStatusHandler
	deletePlatformOrg *command.DeletePlatformOrganizationHandler
	transferOwner    *command.TransferOrgOwnerHandler
	listAuditLogs    *query.ListAuditLogsHandler
	getOrgQuota      *query.GetOrgQuotaHandler
	getOrgQuotaUsage *query.GetOrgQuotaUsageHandler
	updateOrgQuota   *command.UpdateOrgQuotaHandler
	systemRoles      authz.SystemRoleChecker
	orgAccess        authz.OrgPermissionChecker
}

func NewHandler(
	createOrg *command.CreateOrganizationHandler,
	createWs *command.CreateWorkspaceHandler,
	listWs *query.ListWorkspacesHandler,
	listOrgs *query.ListOrganizationsHandler,
	getOrg *query.GetOrganizationHandler,
	listOrgMembers *query.ListOrgMembersHandler,
	myOrgPerms *query.MyOrgPermissionsHandler,
	inviteOrgMember *command.InviteOrgMemberHandler,
	updateOrgRole *command.UpdateOrgMemberRoleHandler,
	removeOrgMember *command.RemoveOrgMemberHandler,
	listPlatformOrgs *query.ListPlatformOrganizationsHandler,
	createPlatformOrg *command.CreatePlatformOrganizationHandler,
	updateOrgStatus *command.UpdateOrganizationStatusHandler,
	deletePlatformOrg *command.DeletePlatformOrganizationHandler,
	transferOwner *command.TransferOrgOwnerHandler,
	listAuditLogs *query.ListAuditLogsHandler,
	getOrgQuota *query.GetOrgQuotaHandler,
	getOrgQuotaUsage *query.GetOrgQuotaUsageHandler,
	updateOrgQuota *command.UpdateOrgQuotaHandler,
	systemRoles authz.SystemRoleChecker,
	orgAccess authz.OrgPermissionChecker,
) *Handler {
	return &Handler{
		createOrg: createOrg, createWs: createWs, listWs: listWs, listOrgs: listOrgs,
		getOrg: getOrg, listOrgMembers: listOrgMembers, myOrgPerms: myOrgPerms,
		inviteOrgMember: inviteOrgMember, updateOrgRole: updateOrgRole, removeOrgMember: removeOrgMember,
		listPlatformOrgs: listPlatformOrgs, createPlatformOrg: createPlatformOrg,
		updateOrgStatus: updateOrgStatus, deletePlatformOrg: deletePlatformOrg, transferOwner: transferOwner, listAuditLogs: listAuditLogs,
		getOrgQuota: getOrgQuota, getOrgQuotaUsage: getOrgQuotaUsage, updateOrgQuota: updateOrgQuota,
		systemRoles: systemRoles, orgAccess: orgAccess,
	}
}

type createOrgRequest struct {
	Name string `json:"name" binding:"required"`
	Slug string `json:"slug" binding:"required"`
}

type createWorkspaceRequest struct {
	Name string `json:"name" binding:"required"`
	Slug string `json:"slug" binding:"required"`
}

type inviteOrgMemberRequest struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role"`
}

type updateOrgMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

func (h *Handler) CreateOrganization(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	var req createOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	org, err := h.createOrg.Handle(c.Request.Context(), memberID, req.Name, req.Slug)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": org.ID.String(), "name": org.Name, "slug": org.Slug})
}

func (h *Handler) ListOrganizations(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	items, err := h.listOrgs.Handle(c.Request.Context(), memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetOrganization(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	org, err := h.getOrg.Handle(c.Request.Context(), orgID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, org)
}

func (h *Handler) ListOrgMembers(c *gin.Context) {
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	items, err := h.listOrgMembers.Handle(c.Request.Context(), orgID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) MyOrgPermissions(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	perms, err := h.myOrgPerms.Handle(c.Request.Context(), orgID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"permissions": perms})
}

func (h *Handler) InviteOrgMember(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	var req inviteOrgMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.inviteOrgMember.Handle(c.Request.Context(), orgID, memberID, req.Email, req.Role)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"member_id": result.MemberID.String(), "email": result.Email,
		"display_name": result.DisplayName, "role": result.Role,
	})
}

func (h *Handler) UpdateOrgMemberRole(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	targetID, err := kernel.ParseMemberID(c.Param("memberId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid member id", http.StatusBadRequest)
		return
	}
	var req updateOrgMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.updateOrgRole.Handle(c.Request.Context(), orgID, memberID, targetID, req.Role); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": true})
}

func (h *Handler) RemoveOrgMember(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	targetID, err := kernel.ParseMemberID(c.Param("memberId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid member id", http.StatusBadRequest)
		return
	}
	if err := h.removeOrgMember.Handle(c.Request.Context(), orgID, memberID, targetID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"removed": true})
}

func (h *Handler) CreateWorkspace(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	if !h.canCreateWorkspace(c, orgID, memberID) {
		httpx.HandleError(c, domain.ErrOrgAccessDenied)
		return
	}
	var req createWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	ws, err := h.createWs.Handle(c.Request.Context(), orgID, memberID, req.Name, req.Slug)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": ws.ID.String(), "name": ws.Name, "slug": ws.Slug})
}

func (h *Handler) canCreateWorkspace(c *gin.Context, orgID kernel.OrgID, memberID kernel.MemberID) bool {
	if h.systemRoles != nil {
		isSuper, err := h.systemRoles.IsSuperAdmin(c.Request.Context(), memberID)
		if err == nil && isSuper {
			return true
		}
	}
	if h.orgAccess == nil {
		return false
	}
	can, err := h.orgAccess.HasOrgPermission(c.Request.Context(), orgID, memberID, authz.PermOrgWorkspaceCreate)
	return err == nil && can
}

func (h *Handler) ListWorkspaces(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	items, err := h.listWs.Handle(c.Request.Context(), memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

type createPlatformOrgRequest struct {
	Name       string `json:"name" binding:"required"`
	Slug       string `json:"slug" binding:"required"`
	OwnerEmail string `json:"owner_email" binding:"omitempty,email"`
}

type updateOrgStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type transferOwnerRequest struct {
	OwnerEmail string `json:"owner_email" binding:"required,email"`
}

func (h *Handler) ListPlatformOrganizations(c *gin.Context) {
	items, err := h.listPlatformOrgs.Handle(c.Request.Context())
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) CreatePlatformOrganization(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	var req createPlatformOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	org, err := h.createPlatformOrg.Handle(c.Request.Context(), memberID, command.CreatePlatformOrganizationInput{
		Name: req.Name, Slug: req.Slug, OwnerEmail: req.OwnerEmail,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": org.ID.String(), "name": org.Name, "slug": org.Slug, "status": org.Status})
}

func (h *Handler) UpdatePlatformOrganizationStatus(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	var req updateOrgStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.updateOrgStatus.Handle(c.Request.Context(), memberID, orgID, req.Status); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": true})
}

func (h *Handler) DeletePlatformOrganization(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	if err := h.deletePlatformOrg.Handle(c.Request.Context(), memberID, orgID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *Handler) TransferPlatformOrgOwner(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	var req transferOwnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.transferOwner.Handle(c.Request.Context(), orgID, memberID, req.OwnerEmail, true)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"previous_owner_id": result.PreviousOwnerID.String(),
		"new_owner_id":      result.NewOwnerID.String(),
		"new_owner_email":   result.NewOwnerEmail,
	})
}

func (h *Handler) TransferOrgOwner(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	var req transferOwnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.transferOwner.Handle(c.Request.Context(), orgID, memberID, req.OwnerEmail, false)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"previous_owner_id": result.PreviousOwnerID.String(),
		"new_owner_id":      result.NewOwnerID.String(),
		"new_owner_email":   result.NewOwnerEmail,
	})
}

func (h *Handler) ListOrgAuditLogs(c *gin.Context) {
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	items, err := h.listAuditLogs.HandleOrg(c.Request.Context(), orgID, parseListLimit(c), parseListOffset(c))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) ListPlatformAuditLogs(c *gin.Context) {
	items, err := h.listAuditLogs.HandlePlatform(c.Request.Context(), parseListLimit(c), parseListOffset(c))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func parseListLimit(c *gin.Context) int {
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 50
}

func parseListOffset(c *gin.Context) int {
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

type updateOrgQuotaRequest struct {
	MaxWorkspaces *int   `json:"max_workspaces"`
	MaxAgents     *int   `json:"max_agents"`
	MaxProjects   *int   `json:"max_projects"`
	TokenBudget   *int64 `json:"token_budget"`
}

func (h *Handler) GetOrgQuota(c *gin.Context) {
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	view, err := h.getOrgQuota.Handle(c.Request.Context(), orgID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, view)
}

func (h *Handler) GetOrgQuotaUsage(c *gin.Context) {
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	view, err := h.getOrgQuotaUsage.Handle(c.Request.Context(), orgID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, view)
}

func (h *Handler) UpdateOrgQuota(c *gin.Context) {
	orgID, ok := httpx.OrgIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_ORG_REQUIRED", "organization required", http.StatusBadRequest)
		return
	}
	var req updateOrgQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	quota, err := h.updateOrgQuota.Handle(c.Request.Context(), orgID, command.UpdateOrgQuotaInput{
		MaxWorkspaces: req.MaxWorkspaces,
		MaxAgents:     req.MaxAgents,
		MaxProjects:   req.MaxProjects,
		TokenBudget:   req.TokenBudget,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"max_workspaces": quota.MaxWorkspaces,
		"max_agents":     quota.MaxAgents,
		"max_projects":   quota.MaxProjects,
		"token_budget":   quota.TokenBudget,
	})
}

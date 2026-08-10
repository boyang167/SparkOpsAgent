package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/governance/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	createRole      *command.CreateRoleHandler
	updateRole      *command.UpdateRoleHandler
	deleteRole      *command.DeleteRoleHandler
	copyRole        *command.CopyRoleHandler
	assignRole      *command.AssignRoleHandler
	inviteMember    *command.InviteWorkspaceMemberHandler
	removeBinding   *command.RemoveRoleBindingHandler
	listRoles       *query.ListRolesHandler
	listRoleTree    *query.ListRoleTreeHandler
	listMembers     *query.ListWorkspaceMembersHandler
	listMemberRoles *query.ListMemberRolesHandler
	checker         *query.Checker
}

func NewHandler(
	createRole *command.CreateRoleHandler,
	updateRole *command.UpdateRoleHandler,
	deleteRole *command.DeleteRoleHandler,
	copyRole *command.CopyRoleHandler,
	assignRole *command.AssignRoleHandler,
	inviteMember *command.InviteWorkspaceMemberHandler,
	removeBinding *command.RemoveRoleBindingHandler,
	listRoles *query.ListRolesHandler,
	listRoleTree *query.ListRoleTreeHandler,
	listMembers *query.ListWorkspaceMembersHandler,
	listMemberRoles *query.ListMemberRolesHandler,
	checker *query.Checker,
) *Handler {
	return &Handler{
		createRole: createRole, updateRole: updateRole, deleteRole: deleteRole, copyRole: copyRole,
		assignRole: assignRole,
		inviteMember: inviteMember, removeBinding: removeBinding,
		listRoles: listRoles, listRoleTree: listRoleTree, listMembers: listMembers,
		listMemberRoles: listMemberRoles, checker: checker,
	}
}

func (h *Handler) Checker() *query.Checker {
	return h.checker
}

type createRoleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Permissions []string `json:"permissions" binding:"required"`
	ParentID    *string  `json:"parent_id"`
}

type copyRoleRequest struct {
	Name string `json:"name"`
}

type assignRoleRequest struct {
	RoleID string `json:"role_id" binding:"required"`
}

type inviteMemberRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type updateRoleRequest struct {
	Permissions []string `json:"permissions" binding:"required"`
}

func (h *Handler) ListRoleTree(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	items, err := h.listRoleTree.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) ListRoles(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	items, err := h.listRoles.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) CreateRole(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	var req createRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	var parentID *kernel.RoleID
	if req.ParentID != nil && *req.ParentID != "" {
		pid, err := kernel.ParseRoleID(*req.ParentID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid parent_id", http.StatusBadRequest)
			return
		}
		parentID = &pid
	}
	role, err := h.createRole.Handle(c.Request.Context(), wsID, req.Name, req.Permissions, parentID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": role.ID.String(), "name": role.Name, "permissions": role.Permissions})
}

func (h *Handler) UpdateRole(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	roleID, err := kernel.ParseRoleID(c.Param("roleId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid role id", http.StatusBadRequest)
		return
	}
	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	role, err := h.updateRole.Handle(c.Request.Context(), wsID, roleID, req.Permissions)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": role.ID.String(), "name": role.Name, "permissions": role.Permissions})
}

func (h *Handler) DeleteRole(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	roleID, err := kernel.ParseRoleID(c.Param("roleId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid role id", http.StatusBadRequest)
		return
	}
	if err := h.deleteRole.Handle(c.Request.Context(), wsID, roleID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *Handler) CopyRole(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	roleID, err := kernel.ParseRoleID(c.Param("roleId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid role id", http.StatusBadRequest)
		return
	}
	var req copyRoleRequest
	_ = c.ShouldBindJSON(&req)
	role, err := h.copyRole.Handle(c.Request.Context(), wsID, roleID, req.Name)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": role.ID.String(), "name": role.Name, "permissions": role.Permissions})
}

func (h *Handler) AssignRole(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	memberID, err := kernel.ParseMemberID(c.Param("memberId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid member id", http.StatusBadRequest)
		return
	}
	var req assignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	roleID, err := kernel.ParseRoleID(req.RoleID)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid role id", http.StatusBadRequest)
		return
	}
	if err := h.assignRole.Handle(c.Request.Context(), wsID, roleID, memberID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"assigned": true})
}

func (h *Handler) ListMembers(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	items, err := h.listMembers.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) ListMemberRoles(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	memberID, err := kernel.ParseMemberID(c.Param("memberId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid member id", http.StatusBadRequest)
		return
	}
	items, err := h.listMemberRoles.Handle(c.Request.Context(), wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) InviteMember(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	var req inviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.inviteMember.Handle(c.Request.Context(), wsID, req.Email)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"member_id":    result.MemberID.String(),
		"email":        result.Email,
		"display_name": result.DisplayName,
	})
}

func (h *Handler) RemoveRoleBinding(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	memberID, err := kernel.ParseMemberID(c.Param("memberId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid member id", http.StatusBadRequest)
		return
	}
	roleID, err := kernel.ParseRoleID(c.Param("roleId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid role id", http.StatusBadRequest)
		return
	}
	if err := h.removeBinding.Handle(c.Request.Context(), wsID, roleID, memberID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"removed": true})
}

func (h *Handler) MyPermissions(c *gin.Context) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	roles, err := h.checker.ListRolesForMember(c.Request.Context(), wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	perms := map[string]bool{}
	for _, role := range roles {
		for _, p := range role.Permissions {
			perms[p] = true
		}
	}
	list := make([]string, 0, len(perms))
	for p := range perms {
		list = append(list, p)
	}
	httpx.OK(c, gin.H{"permissions": list})
}

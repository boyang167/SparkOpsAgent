package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	create  *command.CreateProjectHandler
	update  *command.UpdateProjectHandler
	archive *command.ArchiveProjectHandler
	list    *query.ListProjectsHandler
	get     *query.GetProjectHandler
	createTemplate   *command.CreateTemplateHandler
	listTemplates    *query.ListTemplatesHandler
	createDockerfile *command.CreateDockerfileHandler
	listDockerfiles  *query.ListDockerfilesHandler
	addMember        *command.AddProjectMemberHandler
	updateMemberRole *command.UpdateProjectMemberRoleHandler
	removeMember     *command.RemoveProjectMemberHandler
	listMembers      *query.ListProjectMembersHandler
}

func NewHandler(
	create *command.CreateProjectHandler,
	update *command.UpdateProjectHandler,
	archive *command.ArchiveProjectHandler,
	list *query.ListProjectsHandler,
	get *query.GetProjectHandler,
	createTemplate *command.CreateTemplateHandler,
	listTemplates *query.ListTemplatesHandler,
	createDockerfile *command.CreateDockerfileHandler,
	listDockerfiles *query.ListDockerfilesHandler,
	addMember *command.AddProjectMemberHandler,
	updateMemberRole *command.UpdateProjectMemberRoleHandler,
	removeMember *command.RemoveProjectMemberHandler,
	listMembers *query.ListProjectMembersHandler,
) *Handler {
	return &Handler{
		create: create, update: update, archive: archive, list: list, get: get,
		createTemplate: createTemplate, listTemplates: listTemplates,
		createDockerfile: createDockerfile, listDockerfiles: listDockerfiles,
		addMember: addMember, updateMemberRole: updateMemberRole, removeMember: removeMember,
		listMembers: listMembers,
	}
}

type createProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type updateProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

func (h *Handler) CreateProject(c *gin.Context) {
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
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	project, err := h.create.Handle(c.Request.Context(), wsID, memberID, req.Name, req.Description)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": project.ID.String(), "name": project.Name.String(),
		"description": project.Description, "status": project.Status,
	})
}

func (h *Handler) ListProjects(c *gin.Context) {
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
	items, err := h.list.Handle(c.Request.Context(), wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetProject(c *gin.Context) {
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
	projectID, err := kernel.ParseProjectID(c.Param("projectId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
		return
	}
	item, err := h.get.Handle(c.Request.Context(), wsID, memberID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) UpdateProject(c *gin.Context) {
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
	projectID, err := kernel.ParseProjectID(c.Param("projectId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
		return
	}
	var req updateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	project, err := h.update.Handle(c.Request.Context(), wsID, memberID, projectID, req.Name, req.Description)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": project.ID.String(), "name": project.Name.String(),
		"description": project.Description, "status": project.Status,
	})
}

func (h *Handler) ArchiveProject(c *gin.Context) {
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
	projectID, err := kernel.ParseProjectID(c.Param("projectId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
		return
	}
	if err := h.archive.Handle(c.Request.Context(), wsID, memberID, projectID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

type createTemplateRequest struct {
	Name     string `json:"name" binding:"required"`
	Kind     string `json:"kind"`
	Language string `json:"language"`
	Detail   string `json:"detail"`
	Rules    string `json:"rules"`
}

type createDockerfileRequest struct {
	Name        string `json:"name" binding:"required"`
	Language    string `json:"language"`
	Description string `json:"description"`
	Detail      string `json:"detail"`
	Dockerfile  string `json:"dockerfile" binding:"required"`
	Version     string `json:"version"`
}

func (h *Handler) CreateTemplate(c *gin.Context) {
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
	var req createTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	t, err := h.createTemplate.Handle(c.Request.Context(), wsID, memberID, req.Name, req.Kind, req.Language, req.Detail, req.Rules)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": t.ID.String(), "name": t.Name.String(), "kind": t.Kind,
		"language": t.Language, "detail": t.Detail, "rules": t.Rules,
	})
}

func (h *Handler) ListTemplates(c *gin.Context) {
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
	items, err := h.listTemplates.Handle(c.Request.Context(), wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) CreateDockerfile(c *gin.Context) {
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
	var req createDockerfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	df, err := h.createDockerfile.Handle(c.Request.Context(), wsID, memberID, req.Name, req.Language, req.Description, req.Detail, req.Dockerfile, req.Version)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": df.ID.String(), "name": df.Name.String(), "language": df.Language,
		"description": df.Description, "dockerfile": df.Dockerfile, "version": df.Version,
	})
}

func (h *Handler) ListDockerfiles(c *gin.Context) {
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
	items, err := h.listDockerfiles.Handle(c.Request.Context(), wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

type addProjectMemberRequest struct {
	MemberID string `json:"member_id" binding:"required"`
	Role     string `json:"role"`
}

type updateProjectMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

func (h *Handler) ListProjectMembers(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	projectID, err := kernel.ParseProjectID(c.Param("projectId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
		return
	}
	items, err := h.listMembers.Handle(c.Request.Context(), wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) AddProjectMember(c *gin.Context) {
	actorID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	projectID, err := kernel.ParseProjectID(c.Param("projectId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
		return
	}
	var req addProjectMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	targetID, err := kernel.ParseMemberID(req.MemberID)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid member id", http.StatusBadRequest)
		return
	}
	if err := h.addMember.Handle(c.Request.Context(), wsID, actorID, targetID, projectID, req.Role); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"added": true})
}

func (h *Handler) UpdateProjectMemberRole(c *gin.Context) {
	actorID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	projectID, err := kernel.ParseProjectID(c.Param("projectId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
		return
	}
	targetID, err := kernel.ParseMemberID(c.Param("memberId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid member id", http.StatusBadRequest)
		return
	}
	var req updateProjectMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.updateMemberRole.Handle(c.Request.Context(), wsID, actorID, targetID, projectID, req.Role); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": true})
}

func (h *Handler) RemoveProjectMember(c *gin.Context) {
	actorID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	projectID, err := kernel.ParseProjectID(c.Param("projectId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
		return
	}
	targetID, err := kernel.ParseMemberID(c.Param("memberId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid member id", http.StatusBadRequest)
		return
	}
	if err := h.removeMember.Handle(c.Request.Context(), wsID, actorID, targetID, projectID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"removed": true})
}

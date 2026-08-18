package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
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
	updateTemplate   *command.UpdateTemplateHandler
	deleteTemplate   *command.DeleteTemplateHandler
	listTemplates    *query.ListTemplatesHandler
	getTemplate      *query.GetTemplateHandler
	createSecret     *command.CreateSecretHandler
	updateSecret     *command.UpdateSecretHandler
	deleteSecret     *command.DeleteSecretHandler
	listSecrets      *query.ListSecretsHandler
	getSecret        *query.GetSecretHandler
	testSecret       *query.TestSecretConnectionHandler
	secretCrypto     *application.SecretCrypto
	createDockerfile *command.CreateDockerfileHandler
	listDockerfiles  *query.ListDockerfilesHandler
	addMember        *command.AddProjectMemberHandler
	updateMemberRole *command.UpdateProjectMemberRoleHandler
	removeMember     *command.RemoveProjectMemberHandler
	listMembers      *query.ListProjectMembersHandler
	generateTemplates *query.GenerateProjectTemplatesHandler
	memberLookup     port.MemberLookup
}

func NewHandler(
	create *command.CreateProjectHandler,
	update *command.UpdateProjectHandler,
	archive *command.ArchiveProjectHandler,
	list *query.ListProjectsHandler,
	get *query.GetProjectHandler,
	createTemplate *command.CreateTemplateHandler,
	updateTemplate *command.UpdateTemplateHandler,
	deleteTemplate *command.DeleteTemplateHandler,
	listTemplates *query.ListTemplatesHandler,
	getTemplate *query.GetTemplateHandler,
	createSecret *command.CreateSecretHandler,
	updateSecret *command.UpdateSecretHandler,
	deleteSecret *command.DeleteSecretHandler,
	listSecrets *query.ListSecretsHandler,
	getSecret *query.GetSecretHandler,
	testSecret *query.TestSecretConnectionHandler,
	secretCrypto *application.SecretCrypto,
	createDockerfile *command.CreateDockerfileHandler,
	listDockerfiles *query.ListDockerfilesHandler,
	addMember *command.AddProjectMemberHandler,
	updateMemberRole *command.UpdateProjectMemberRoleHandler,
	removeMember *command.RemoveProjectMemberHandler,
	listMembers *query.ListProjectMembersHandler,
	generateTemplates *query.GenerateProjectTemplatesHandler,
	memberLookup port.MemberLookup,
) *Handler {
	return &Handler{
		create: create, update: update, archive: archive, list: list, get: get,
		createTemplate: createTemplate, updateTemplate: updateTemplate, deleteTemplate: deleteTemplate,
		listTemplates: listTemplates, getTemplate: getTemplate,
		createSecret: createSecret, updateSecret: updateSecret, deleteSecret: deleteSecret,
		listSecrets: listSecrets, getSecret: getSecret, testSecret: testSecret,
		secretCrypto: secretCrypto,
		createDockerfile: createDockerfile, listDockerfiles: listDockerfiles,
		addMember: addMember, updateMemberRole: updateMemberRole, removeMember: removeMember,
		listMembers: listMembers, generateTemplates: generateTemplates, memberLookup: memberLookup,
	}
}

type projectResourcesRequest struct {
	CPURequest             string              `json:"cpu_request"`
	CPULimit               string              `json:"cpu_limit"`
	MemoryRequest          string              `json:"memory_request"`
	MemoryLimit            string              `json:"memory_limit"`
	Replicas               int                 `json:"replicas"`
	Ports                  []model.ProjectPort `json:"ports"`
	Command                string              `json:"command"`
	Args                   string              `json:"args"`
	ProjectConfig          string              `json:"project_config"`
	ServiceTemplateID      string              `json:"service_template_id"`
	DeploymentTemplateID  string              `json:"deployment_template_id"`
	JenkinsfileTemplateID string              `json:"jenkinsfile_template_id"`
	RegistryType           string              `json:"registry_type"`
	RegistrySecretID       string              `json:"registry_secret_id"`
	SonarqubeSecretID      string              `json:"sonarqube_secret_id"`
	EnableStaticScan       bool                `json:"enable_static_scan"`
}

type projectPayload struct {
	Name           string                  `json:"name" binding:"required"`
	DisplayName    string                  `json:"display_name"`
	Description    string                  `json:"description"`
	Namespace      string                  `json:"namespace"`
	Language       string                  `json:"language"`
	GitURL         string                  `json:"git_url"`
	SecretID       string                  `json:"secret_id"`
	GitRefType     string                  `json:"git_ref_type"`
	GitRefValue    string                  `json:"git_ref_value"`
	DockerfilePath string                  `json:"dockerfile_path"`
	TemplateID     string                  `json:"template_id"`
	Resources      projectResourcesRequest `json:"resources"`
}

func toProjectInput(req projectPayload) model.CreateProjectInput {
	return model.CreateProjectInput{
		Name:           req.Name,
		DisplayName:    req.DisplayName,
		Description:    req.Description,
		Namespace:      req.Namespace,
		Language:       req.Language,
		GitURL:         req.GitURL,
		SecretID:       req.SecretID,
		GitRefType:     req.GitRefType,
		GitRefValue:    req.GitRefValue,
		DockerfilePath: req.DockerfilePath,
		TemplateID:     req.TemplateID,
		Resources: model.ProjectResources{
			CPURequest:             req.Resources.CPURequest,
			CPULimit:               req.Resources.CPULimit,
			MemoryRequest:          req.Resources.MemoryRequest,
			MemoryLimit:            req.Resources.MemoryLimit,
			Replicas:               req.Resources.Replicas,
			Ports:                  req.Resources.Ports,
			Command:                req.Resources.Command,
			Args:                   req.Resources.Args,
			ProjectConfig:          req.Resources.ProjectConfig,
			ServiceTemplateID:      req.Resources.ServiceTemplateID,
			DeploymentTemplateID:  req.Resources.DeploymentTemplateID,
			JenkinsfileTemplateID: req.Resources.JenkinsfileTemplateID,
			RegistryType:           req.Resources.RegistryType,
			RegistrySecretID:       req.Resources.RegistrySecretID,
			SonarqubeSecretID:      req.Resources.SonarqubeSecretID,
			EnableStaticScan:       req.Resources.EnableStaticScan,
		},
	}
}

func projectResponse(project *model.Project) gin.H {
	view := query.ToView(project)
	return gin.H{
		"id":              view.ID,
		"name":            view.Name,
		"display_name":    view.DisplayName,
		"description":     view.Description,
		"namespace":       view.Namespace,
		"language":        view.Language,
		"git_url":         view.GitURL,
		"secret_id":       view.SecretID,
		"git_ref_type":    view.GitRefType,
		"git_ref_value":   view.GitRefValue,
		"dockerfile_path": view.DockerfilePath,
		"template_id":     view.TemplateID,
		"resources":       view.Resources,
		"status":          view.Status,
		"created_at":      view.CreatedAt,
	}
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
	var req projectPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	project, err := h.create.Handle(c.Request.Context(), wsID, memberID, toProjectInput(req))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, projectResponse(project))
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

func (h *Handler) GenerateProjectTemplates(c *gin.Context) {
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
	items, err := h.generateTemplates.Handle(c.Request.Context(), wsID, memberID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
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
	var req projectPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	project, err := h.update.Handle(c.Request.Context(), wsID, memberID, projectID, toProjectInput(req))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, projectResponse(project))
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
	Status   string `json:"status"`
	IsPublic bool   `json:"is_public"`
}

type updateTemplateRequest struct {
	Name     string `json:"name" binding:"required"`
	Kind     string `json:"kind"`
	Language string `json:"language"`
	Detail   string `json:"detail"`
	Rules    string `json:"rules"`
	Status   string `json:"status"`
	IsPublic bool   `json:"is_public"`
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
	t, err := h.createTemplate.Handle(c.Request.Context(), wsID, memberID, req.Name, req.Kind, req.Language, req.Detail, req.Rules, model.TemplateStatus(req.Status), req.IsPublic)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	item, err := query.EnrichTemplateMap(c.Request.Context(), h.memberLookup, t)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) GetTemplate(c *gin.Context) {
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
	templateID, err := kernel.ParseTemplateID(c.Param("templateId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid template id", http.StatusBadRequest)
		return
	}
	item, err := h.getTemplate.Handle(c.Request.Context(), wsID, memberID, templateID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) UpdateTemplate(c *gin.Context) {
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
	templateID, err := kernel.ParseTemplateID(c.Param("templateId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid template id", http.StatusBadRequest)
		return
	}
	var req updateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	t, err := h.updateTemplate.Handle(c.Request.Context(), wsID, memberID, templateID, req.Name, req.Kind, req.Language, req.Detail, req.Rules, model.TemplateStatus(req.Status), req.IsPublic)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	item, err := query.EnrichTemplateMap(c.Request.Context(), h.memberLookup, t)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) DeleteTemplate(c *gin.Context) {
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
	templateID, err := kernel.ParseTemplateID(c.Param("templateId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid template id", http.StatusBadRequest)
		return
	}
	if err := h.deleteTemplate.Handle(c.Request.Context(), wsID, memberID, templateID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
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

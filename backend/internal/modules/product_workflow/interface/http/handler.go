package http

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	listTemplates    *query.ListWorkflowTemplatesHandler
	getTemplate      *query.GetWorkflowTemplateHandler
	previewTemplate  *query.PreviewWorkflowTemplateHandler
	listWorkspaceWfs *query.ListWorkspaceWorkflowsHandler
	createTemplate   *command.CreateWorkflowTemplateHandler
	updateTemplate   *command.UpdateWorkflowTemplateHandler
	publishTemplate  *command.PublishWorkflowTemplateHandler
	workflows        WorkflowHandlers
}

func NewHandler(
	listTemplates *query.ListWorkflowTemplatesHandler,
	getTemplate *query.GetWorkflowTemplateHandler,
	previewTemplate *query.PreviewWorkflowTemplateHandler,
	listWorkspaceWfs *query.ListWorkspaceWorkflowsHandler,
	createTemplate *command.CreateWorkflowTemplateHandler,
	updateTemplate *command.UpdateWorkflowTemplateHandler,
	publishTemplate *command.PublishWorkflowTemplateHandler,
	workflows WorkflowHandlers,
) *Handler {
	return &Handler{
		listTemplates: listTemplates, getTemplate: getTemplate, previewTemplate: previewTemplate,
		listWorkspaceWfs: listWorkspaceWfs, createTemplate: createTemplate, updateTemplate: updateTemplate,
		publishTemplate: publishTemplate, workflows: workflows,
	}
}

func memberAndWorkspace(c *gin.Context) (kernel.WorkspaceID, kernel.MemberID, bool) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	return wsID, memberID, true
}

func (h *Handler) ListTemplates(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	items, err := h.listTemplates.Handle(c.Request.Context(), wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetTemplate(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	templateID, err := kernel.ParseWorkflowTemplateID(c.Param("templateId"))
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

type previewTemplateRequest struct {
	KeyVals map[string]string `json:"key_vals"`
}

func (h *Handler) ListWorkspaceWorkflows(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	items, err := h.listWorkspaceWfs.Handle(c.Request.Context(), wsID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

type createTemplateRequest struct {
	Name        string          `json:"name" binding:"required"`
	Key         string          `json:"key" binding:"required"`
	Description string          `json:"description"`
	Definition  json.RawMessage `json:"definition" binding:"required"`
}

func (h *Handler) CreateTemplate(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	var req createTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	tpl, err := h.createTemplate.Handle(c.Request.Context(), wsID, memberID, command.CreateWorkflowTemplateInput{
		Name: req.Name, Key: req.Key, Description: req.Description, Definition: req.Definition,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": tpl.ID.String(), "status": tpl.Status, "version": tpl.Version})
}

type updateTemplateRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Definition  json.RawMessage `json:"definition"`
}

func (h *Handler) UpdateTemplate(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	templateID, err := kernel.ParseWorkflowTemplateID(c.Param("templateId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid template id", http.StatusBadRequest)
		return
	}
	var req updateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	tpl, err := h.updateTemplate.Handle(c.Request.Context(), wsID, memberID, templateID, command.UpdateWorkflowTemplateInput{
		Name: req.Name, Description: req.Description, Definition: req.Definition,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": tpl.ID.String(), "status": tpl.Status, "version": tpl.Version})
}

func (h *Handler) PublishTemplate(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	templateID, err := kernel.ParseWorkflowTemplateID(c.Param("templateId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid template id", http.StatusBadRequest)
		return
	}
	tpl, err := h.publishTemplate.Handle(c.Request.Context(), wsID, memberID, templateID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": tpl.ID.String(), "status": tpl.Status, "version": tpl.Version})
}

func (h *Handler) PreviewTemplate(c *gin.Context) {
	templateID, err := kernel.ParseWorkflowTemplateID(c.Param("templateId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid template id", http.StatusBadRequest)
		return
	}
	var req previewTemplateRequest
	_ = c.ShouldBindJSON(&req)
	item, err := h.previewTemplate.Handle(c.Request.Context(), templateID, req.KeyVals)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

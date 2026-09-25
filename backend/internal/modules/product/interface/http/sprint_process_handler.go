package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func (h *Handler) ListSprintProcesses(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	items, err := h.sprints.listProcesses.Handle(c.Request.Context(), productID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, sprintProcessVM(item))
	}
	httpx.OK(c, gin.H{"items": out})
}

type createSprintProcessRequest struct {
	Name        string              `json:"name" binding:"required"`
	TemplateKey string              `json:"template_key"`
	Description string              `json:"description"`
	Stages      []model.SprintStage `json:"stages" binding:"required"`
	IsDefault   bool                `json:"is_default"`
}

func (h *Handler) CreateSprintProcess(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	var req createSprintProcessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	process, err := h.sprints.createProcess.Handle(c.Request.Context(), productID, memberID, command.CreateSprintProcessInput{
		Name: req.Name, TemplateKey: req.TemplateKey, Description: req.Description, Stages: req.Stages, IsDefault: req.IsDefault,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, sprintProcessVM(process))
}

type updateSprintProcessByIDRequest struct {
	Name        string              `json:"name"`
	TemplateKey string              `json:"template_key"`
	Description string              `json:"description"`
	Stages      []model.SprintStage `json:"stages"`
}

func (h *Handler) UpdateSprintProcessByID(c *gin.Context) {
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	processID, err := kernel.ParseSprintProcessID(c.Param("processId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid process id", http.StatusBadRequest)
		return
	}
	var req updateSprintProcessByIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	process, err := h.sprints.updateProcessByID.Handle(c.Request.Context(), productID, processID, command.UpdateSprintProcessByIDInput{
		Name: req.Name, TemplateKey: req.TemplateKey, Description: req.Description, Stages: req.Stages,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, sprintProcessVM(process))
}

func (h *Handler) DeleteSprintProcess(c *gin.Context) {
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	processID, err := kernel.ParseSprintProcessID(c.Param("processId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid process id", http.StatusBadRequest)
		return
	}
	if err := h.sprints.deleteProcess.Handle(c.Request.Context(), productID, processID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type importWorkspaceTemplateRequest struct {
	WorkspaceTemplateID string `json:"workspace_template_id" binding:"required"`
	Name                string `json:"name"`
}

func (h *Handler) ImportWorkspaceSprintTemplate(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	var req importWorkspaceTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	tplID, err := kernel.ParseWorkspaceSprintProcessTemplateID(req.WorkspaceTemplateID)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace_template_id", http.StatusBadRequest)
		return
	}
	process, err := h.sprints.importWsTemplate.Handle(c.Request.Context(), wsID, productID, memberID, command.ImportWorkspaceTemplateInput{
		WorkspaceTemplateID: tplID, Name: req.Name,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, sprintProcessVM(process))
}

func (h *Handler) ListWorkspaceSprintTemplates(c *gin.Context) {
	if _, _, ok := memberAndWorkspace(c); !ok {
		return
	}
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	items, err := h.sprints.listWsTemplates.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, workspaceSprintTemplateVM(item))
	}
	httpx.OK(c, gin.H{"items": out})
}

type createWorkspaceSprintTemplateRequest struct {
	Name        string              `json:"name" binding:"required"`
	TemplateKey string              `json:"template_key"`
	Description string              `json:"description"`
	Stages      []model.SprintStage `json:"stages" binding:"required"`
}

func (h *Handler) CreateWorkspaceSprintTemplate(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	var req createWorkspaceSprintTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	tpl, err := h.sprints.createWsTemplate.Handle(c.Request.Context(), wsID, memberID, command.CreateWorkspaceSprintTemplateInput{
		Name: req.Name, TemplateKey: req.TemplateKey, Description: req.Description, Stages: req.Stages,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, workspaceSprintTemplateVM(tpl))
}

func (h *Handler) DeleteWorkspaceSprintTemplate(c *gin.Context) {
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	templateID, err := kernel.ParseWorkspaceSprintProcessTemplateID(c.Param("templateId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid template id", http.StatusBadRequest)
		return
	}
	if err := h.sprints.deleteWsTemplate.Handle(c.Request.Context(), wsID, templateID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/sprint_process/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	getOrSeed      *command.GetOrSeedSprintProcessHandler
	upsertProcess  *command.UpsertSprintProcessHandler
	listProcesses  *command.ListSprintProcessesHandler
	createProcess  *command.CreateSprintProcessHandler
	updateProcess  *command.UpdateSprintProcessByIDHandler
	deleteProcess  *command.DeleteSprintProcessHandler
	importWs       *command.ImportWorkspaceTemplateToProductHandler
	listWsTpl      *command.ListWorkspaceSprintTemplatesHandler
	createWsTpl    *command.CreateWorkspaceSprintTemplateHandler
	updateWsTpl    *command.UpdateWorkspaceSprintTemplateHandler
	deleteWsTpl    *command.DeleteWorkspaceSprintTemplateHandler
}

func NewHandler(
	getOrSeed *command.GetOrSeedSprintProcessHandler,
	upsertProcess *command.UpsertSprintProcessHandler,
	listProcesses *command.ListSprintProcessesHandler,
	createProcess *command.CreateSprintProcessHandler,
	updateProcess *command.UpdateSprintProcessByIDHandler,
	deleteProcess *command.DeleteSprintProcessHandler,
	importWs *command.ImportWorkspaceTemplateToProductHandler,
	listWsTpl *command.ListWorkspaceSprintTemplatesHandler,
	createWsTpl *command.CreateWorkspaceSprintTemplateHandler,
	updateWsTpl *command.UpdateWorkspaceSprintTemplateHandler,
	deleteWsTpl *command.DeleteWorkspaceSprintTemplateHandler,
) *Handler {
	return &Handler{
		getOrSeed: getOrSeed, upsertProcess: upsertProcess, listProcesses: listProcesses,
		createProcess: createProcess, updateProcess: updateProcess, deleteProcess: deleteProcess,
		importWs: importWs, listWsTpl: listWsTpl, createWsTpl: createWsTpl,
		updateWsTpl: updateWsTpl, deleteWsTpl: deleteWsTpl,
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

func (h *Handler) GetSprintProcess(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	process, err := h.getOrSeed.Handle(c.Request.Context(), productID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, sprintProcessVM(process))
}

type upsertSprintProcessRequest struct {
	Name   string              `json:"name" binding:"required"`
	Stages []model.SprintStage `json:"stages" binding:"required"`
}

func (h *Handler) UpsertSprintProcess(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	var req upsertSprintProcessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	process, err := h.upsertProcess.Handle(c.Request.Context(), productID, memberID, command.UpsertSprintProcessInput{
		Name: req.Name, Stages: req.Stages,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, sprintProcessVM(process))
}

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
	items, err := h.listProcesses.Handle(c.Request.Context(), productID, memberID)
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
	process, err := h.createProcess.Handle(c.Request.Context(), productID, memberID, command.CreateSprintProcessInput{
		Name: req.Name, TemplateKey: req.TemplateKey, Description: req.Description,
		Stages: req.Stages, IsDefault: req.IsDefault,
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
	process, err := h.updateProcess.Handle(c.Request.Context(), productID, processID, command.UpdateSprintProcessByIDInput{
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
	if err := h.deleteProcess.Handle(c.Request.Context(), productID, processID); err != nil {
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
	process, err := h.importWs.Handle(c.Request.Context(), wsID, productID, memberID, command.ImportWorkspaceTemplateInput{
		WorkspaceTemplateID: tplID, Name: req.Name,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, sprintProcessVM(process))
}

func (h *Handler) ListWorkspaceSprintTemplates(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	items, err := h.listWsTpl.Handle(c.Request.Context(), wsID, memberID)
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
	tpl, err := h.createWsTpl.Handle(c.Request.Context(), wsID, memberID, command.CreateWorkspaceSprintTemplateInput{
		Name: req.Name, TemplateKey: req.TemplateKey, Description: req.Description, Stages: req.Stages,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, workspaceSprintTemplateVM(tpl))
}

type updateWorkspaceSprintTemplateRequest struct {
	Name        string              `json:"name"`
	TemplateKey string              `json:"template_key"`
	Description string              `json:"description"`
	Stages      []model.SprintStage `json:"stages"`
}

func (h *Handler) UpdateWorkspaceSprintTemplate(c *gin.Context) {
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
	var req updateWorkspaceSprintTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	tpl, err := h.updateWsTpl.Handle(c.Request.Context(), wsID, templateID, command.UpdateWorkspaceSprintTemplateInput{
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
	if err := h.deleteWsTpl.Handle(c.Request.Context(), wsID, templateID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func sprintProcessVM(p *model.SprintProcess) gin.H {
	return gin.H{
		"id": p.ID.String(), "product_id": p.ProductID.String(), "name": p.Name,
		"template_key": p.TemplateKey, "description": p.Description,
		"version": p.Version, "is_default": p.IsDefault, "stages": p.Stages,
		"created_by": p.CreatedBy.String(),
		"created_at": p.CreatedAt.Format(time.RFC3339),
		"updated_at": p.UpdatedAt.Format(time.RFC3339),
	}
}

func workspaceSprintTemplateVM(t *model.WorkspaceSprintProcessTemplate) gin.H {
	return gin.H{
		"id": t.ID.String(), "workspace_id": t.WorkspaceID.String(), "name": t.Name,
		"template_key": t.TemplateKey, "description": t.Description, "stages": t.Stages,
		"created_by": t.CreatedBy.String(),
		"created_at": t.CreatedAt.Format(time.RFC3339),
		"updated_at": t.UpdatedAt.Format(time.RFC3339),
	}
}

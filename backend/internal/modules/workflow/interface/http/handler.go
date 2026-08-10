package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	create    *command.CreateWorkflowHandler
	update    *command.UpdateWorkflowHandler
	activate  *command.ActivateWorkflowHandler
	archive   *command.ArchiveWorkflowHandler
	startRun  *command.StartWorkflowRunHandler
	list      *query.ListWorkflowsHandler
	get       *query.GetWorkflowHandler
	listRuns  *query.ListWorkflowRunsHandler
	getRun    *query.GetWorkflowRunHandler
}

func NewHandler(
	create *command.CreateWorkflowHandler,
	update *command.UpdateWorkflowHandler,
	activate *command.ActivateWorkflowHandler,
	archive *command.ArchiveWorkflowHandler,
	startRun *command.StartWorkflowRunHandler,
	list *query.ListWorkflowsHandler,
	get *query.GetWorkflowHandler,
	listRuns *query.ListWorkflowRunsHandler,
	getRun *query.GetWorkflowRunHandler,
) *Handler {
	return &Handler{
		create: create, update: update, activate: activate, archive: archive,
		startRun: startRun, list: list, get: get, listRuns: listRuns, getRun: getRun,
	}
}

type createWorkflowRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Definition  string `json:"definition"`
}

type updateWorkflowRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Definition  string `json:"definition"`
}

type startWorkflowRunRequest struct {
	Input string `json:"input"`
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

func (h *Handler) CreateWorkflow(c *gin.Context) {
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
	var req createWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	workflow, err := h.create.Handle(c.Request.Context(), wsID, projectID, req.Name, req.Description, req.Definition)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": workflow.ID.String(), "name": workflow.Name.String(),
		"description": workflow.Description, "status": workflow.Status,
		"definition": workflow.Definition,
	})
}

func (h *Handler) ListWorkflows(c *gin.Context) {
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
	items, err := h.list.Handle(c.Request.Context(), wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetWorkflow(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	workflowID, err := kernel.ParseWorkflowID(c.Param("workflowId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workflow id", http.StatusBadRequest)
		return
	}
	item, err := h.get.Handle(c.Request.Context(), wsID, workflowID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) UpdateWorkflow(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	workflowID, err := kernel.ParseWorkflowID(c.Param("workflowId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workflow id", http.StatusBadRequest)
		return
	}
	var req updateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	workflow, err := h.update.Handle(c.Request.Context(), wsID, workflowID, req.Name, req.Description, req.Definition)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": workflow.ID.String(), "name": workflow.Name.String(),
		"description": workflow.Description, "status": workflow.Status,
		"definition": workflow.Definition,
	})
}

func (h *Handler) ArchiveWorkflow(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	workflowID, err := kernel.ParseWorkflowID(c.Param("workflowId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workflow id", http.StatusBadRequest)
		return
	}
	if err := h.archive.Handle(c.Request.Context(), wsID, workflowID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *Handler) ActivateWorkflow(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	workflowID, err := kernel.ParseWorkflowID(c.Param("workflowId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workflow id", http.StatusBadRequest)
		return
	}
	workflow, err := h.activate.Handle(c.Request.Context(), wsID, workflowID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": workflow.ID.String(), "name": workflow.Name.String(),
		"description": workflow.Description, "status": workflow.Status,
		"definition": workflow.Definition, "project_id": workflow.ProjectID.String(),
	})
}

func (h *Handler) StartWorkflowRun(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	workflowID, err := kernel.ParseWorkflowID(c.Param("workflowId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workflow id", http.StatusBadRequest)
		return
	}
	var req startWorkflowRunRequest
	_ = c.ShouldBindJSON(&req)
	run, err := h.startRun.Handle(c.Request.Context(), wsID, memberID, workflowID, req.Input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": run.ID.String(), "workflow_id": run.WorkflowID.String(),
		"status": run.Status, "output": run.Output, "error": run.Error,
	})
}

func (h *Handler) ListWorkflowRuns(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	workflowID, err := kernel.ParseWorkflowID(c.Param("workflowId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workflow id", http.StatusBadRequest)
		return
	}
	items, err := h.listRuns.Handle(c.Request.Context(), wsID, workflowID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetWorkflowRun(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	runID, err := kernel.ParseWorkflowRunID(c.Param("runId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workflow run id", http.StatusBadRequest)
		return
	}
	item, err := h.getRun.Handle(c.Request.Context(), wsID, runID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

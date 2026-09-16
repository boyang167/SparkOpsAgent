package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkflowHandlers struct {
	CreateFromTemplate *command.CreateWorkflowFromTemplateHandler
	StartRun           *command.StartWorkflowRunHandler
	ListWorkflows      *query.ListProductWorkflowsHandler
	ListRuns           *query.ListWorkflowRunsHandler
	GetRun             *query.GetWorkflowRunHandler
	ApproveJob         *command.ApproveWorkflowJobHandler
	RejectJob          *command.RejectWorkflowJobHandler
}

type createFromTemplateRequest struct {
	TemplateID string            `json:"template_id" binding:"required"`
	Name       string            `json:"name"`
	KeyVals    map[string]string `json:"key_vals"`
	ProjectIDs []string          `json:"project_ids" binding:"required"`
}

func (h *Handler) ListProductWorkflows(c *gin.Context) {
	wsID, _, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	items, err := h.workflows.ListWorkflows.Handle(c.Request.Context(), wsID, productID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) CreateWorkflowFromTemplate(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	var req createFromTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	templateID, err := kernel.ParseWorkflowTemplateID(req.TemplateID)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid template id", http.StatusBadRequest)
		return
	}
	projectIDs := make([]kernel.ProjectID, 0, len(req.ProjectIDs))
	for _, raw := range req.ProjectIDs {
		pid, err := kernel.ParseProjectID(raw)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
			return
		}
		projectIDs = append(projectIDs, pid)
	}
	wf, err := h.workflows.CreateFromTemplate.Handle(c.Request.Context(), wsID, productID, memberID, command.CreateWorkflowFromTemplateInput{
		TemplateID: templateID, Name: req.Name, KeyVals: req.KeyVals, ProjectIDs: projectIDs,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": wf.ID.String(), "name": wf.Name, "key": wf.Key, "status": wf.Status})
}

func (h *Handler) StartWorkflowRun(c *gin.Context) {
	wsID, _, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	workflowID, err := kernel.ParseProductWorkflowID(c.Param("workflowId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workflow id", http.StatusBadRequest)
		return
	}
	run, err := h.workflows.StartRun.Handle(c.Request.Context(), wsID, productID, workflowID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": run.ID.String(), "status": run.Status, "stage_runs": run.StageRuns})
}

func (h *Handler) ListWorkflowRuns(c *gin.Context) {
	wsID, _, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	workflowID, err := kernel.ParseProductWorkflowID(c.Param("workflowId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workflow id", http.StatusBadRequest)
		return
	}
	items, err := h.workflows.ListRuns.Handle(c.Request.Context(), wsID, productID, workflowID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetWorkflowRun(c *gin.Context) {
	wsID, _, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	runID, err := kernel.ParseProductWorkflowRunID(c.Param("runId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid run id", http.StatusBadRequest)
		return
	}
	item, err := h.workflows.GetRun.Handle(c.Request.Context(), wsID, productID, runID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

type rejectWorkflowJobRequest struct {
	Message string `json:"message"`
}

func (h *Handler) ApproveWorkflowJob(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	runID, err := kernel.ParseProductWorkflowRunID(c.Param("runId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid run id", http.StatusBadRequest)
		return
	}
	jobName := c.Param("jobName")
	if jobName == "" {
		httpx.Fail(c, "INVALID_REQUEST", "job name required", http.StatusBadRequest)
		return
	}
	if err := h.workflows.ApproveJob.Handle(c.Request.Context(), wsID, productID, runID, jobName, memberID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"ok": true})
}

func (h *Handler) RejectWorkflowJob(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	runID, err := kernel.ParseProductWorkflowRunID(c.Param("runId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid run id", http.StatusBadRequest)
		return
	}
	jobName := c.Param("jobName")
	if jobName == "" {
		httpx.Fail(c, "INVALID_REQUEST", "job name required", http.StatusBadRequest)
		return
	}
	var req rejectWorkflowJobRequest
	_ = c.ShouldBindJSON(&req)
	if err := h.workflows.RejectJob.Handle(c.Request.Context(), wsID, productID, runID, jobName, memberID, req.Message); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"ok": true})
}

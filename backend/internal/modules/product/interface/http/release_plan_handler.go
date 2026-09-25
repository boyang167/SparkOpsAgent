package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ReleasePlanHandlers struct {
	createPlan     *command.CreateReleasePlanHandler
	addJob         *command.AddReleasePlanJobHandler
	finishPlanning *command.FinishReleasePlanPlanningHandler
	approvePlan    *command.ApproveReleasePlanHandler
	rejectPlan     *command.RejectReleasePlanHandler
	markJobDone    *command.MarkReleasePlanJobDoneHandler
	skipJob        *command.SkipReleasePlanJobHandler
	executePlan    *command.ExecuteReleasePlanHandler
	retryJob       *command.RetryReleasePlanJobHandler
	listPlans      *query.ListReleasePlansHandler
	getPlan        *query.GetReleasePlanHandler
}

func NewReleasePlanHandlers(
	createPlan *command.CreateReleasePlanHandler,
	addJob *command.AddReleasePlanJobHandler,
	finishPlanning *command.FinishReleasePlanPlanningHandler,
	approvePlan *command.ApproveReleasePlanHandler,
	rejectPlan *command.RejectReleasePlanHandler,
	markJobDone *command.MarkReleasePlanJobDoneHandler,
	skipJob *command.SkipReleasePlanJobHandler,
	executePlan *command.ExecuteReleasePlanHandler,
	retryJob *command.RetryReleasePlanJobHandler,
	listPlans *query.ListReleasePlansHandler,
	getPlan *query.GetReleasePlanHandler,
) ReleasePlanHandlers {
	return ReleasePlanHandlers{
		createPlan: createPlan, addJob: addJob, finishPlanning: finishPlanning,
		approvePlan: approvePlan, rejectPlan: rejectPlan, markJobDone: markJobDone, skipJob: skipJob,
		executePlan: executePlan, retryJob: retryJob, listPlans: listPlans, getPlan: getPlan,
	}
}

func (h *Handler) ListReleasePlans(c *gin.Context) {
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	var sprintID *kernel.SprintID
	if raw := c.Query("sprint_id"); raw != "" {
		parsed, err := kernel.ParseSprintID(raw)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid sprint_id", http.StatusBadRequest)
			return
		}
		sprintID = &parsed
	}
	items, err := h.releasePlans.listPlans.Handle(c.Request.Context(), productID, sprintID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetReleasePlan(c *gin.Context) {
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	item, err := h.releasePlans.getPlan.Handle(c.Request.Context(), productID, planID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

type createReleasePlanRequest struct {
	Name               string `json:"name" binding:"required"`
	Description        string `json:"description"`
	ReleaseWindowStart string `json:"release_window_start" binding:"required"`
	ReleaseWindowEnd   string `json:"release_window_end" binding:"required"`
	ApprovalEnabled    bool   `json:"approval_enabled"`
	SprintID           string `json:"sprint_id"`
}

func (h *Handler) CreateReleasePlan(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	_ = wsID
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	var req createReleasePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	start, err := time.Parse(time.RFC3339, req.ReleaseWindowStart)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid release_window_start", http.StatusBadRequest)
		return
	}
	end, err := time.Parse(time.RFC3339, req.ReleaseWindowEnd)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid release_window_end", http.StatusBadRequest)
		return
	}
	input := command.CreateReleasePlanInput{
		Name: req.Name, Description: req.Description,
		ReleaseWindowStart: start, ReleaseWindowEnd: end, ApprovalEnabled: req.ApprovalEnabled,
	}
	if req.SprintID != "" {
		sid, err := kernel.ParseSprintID(req.SprintID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid sprint_id", http.StatusBadRequest)
			return
		}
		input.SprintID = &sid
	}
	plan, err := h.releasePlans.createPlan.Handle(c.Request.Context(), productID, memberID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

type addReleasePlanJobRequest struct {
	Name        string          `json:"name" binding:"required"`
	Type        string          `json:"type" binding:"required"`
	TextContent string          `json:"text_content"`
	SortOrder   int             `json:"sort_order"`
	WorkflowRef json.RawMessage `json:"workflow_ref"`
}

func (h *Handler) AddReleasePlanJob(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	var req addReleasePlanJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	jobType := model.ReleasePlanJobType(req.Type)
	if jobType != model.ReleasePlanJobTypeText && jobType != model.ReleasePlanJobTypeWorkflow {
		httpx.Fail(c, "INVALID_REQUEST", "invalid job type", http.StatusBadRequest)
		return
	}
	job, err := h.releasePlans.addJob.Handle(c.Request.Context(), productID, planID, memberID, command.AddReleasePlanJobInput{
		Name: req.Name, Type: jobType, TextContent: req.TextContent, SortOrder: req.SortOrder, WorkflowRef: req.WorkflowRef,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": job.ID.String(), "status": job.Status})
}

func (h *Handler) FinishReleasePlanPlanning(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	plan, err := h.releasePlans.finishPlanning.Handle(c.Request.Context(), productID, planID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func (h *Handler) ApproveReleasePlan(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	plan, err := h.releasePlans.approvePlan.Handle(c.Request.Context(), productID, planID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func (h *Handler) RejectReleasePlan(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	plan, err := h.releasePlans.rejectPlan.Handle(c.Request.Context(), productID, planID, memberID, req.Reason)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func (h *Handler) MarkReleasePlanJobDone(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	jobID, err := kernel.ParseReleasePlanJobID(c.Param("jobId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid job id", http.StatusBadRequest)
		return
	}
	plan, err := h.releasePlans.markJobDone.Handle(c.Request.Context(), productID, planID, jobID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func (h *Handler) ExecuteReleasePlan(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	plan, err := h.releasePlans.executePlan.Handle(c.Request.Context(), productID, planID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func (h *Handler) RetryReleasePlanJob(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	jobID, err := kernel.ParseReleasePlanJobID(c.Param("jobId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid job id", http.StatusBadRequest)
		return
	}
	plan, err := h.releasePlans.retryJob.Handle(c.Request.Context(), productID, planID, jobID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func (h *Handler) SkipReleasePlanJob(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	jobID, err := kernel.ParseReleasePlanJobID(c.Param("jobId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid job id", http.StatusBadRequest)
		return
	}
	plan, err := h.releasePlans.skipJob.Handle(c.Request.Context(), productID, planID, jobID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func parseProductReleasePlanIDs(c *gin.Context) (kernel.ProductID, kernel.ReleasePlanID, bool) {
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return kernel.ProductID{}, kernel.ReleasePlanID{}, false
	}
	planID, err := kernel.ParseReleasePlanID(c.Param("planId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid release plan id", http.StatusBadRequest)
		return kernel.ProductID{}, kernel.ReleasePlanID{}, false
	}
	return productID, planID, true
}

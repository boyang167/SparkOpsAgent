package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ReleasePlanHandlers struct {
	createPlan       *command.CreateReleasePlanHandler
	updatePlan       *command.UpdateReleasePlanHandler
	cancelPlan       *command.CancelReleasePlanHandler
	deletePlan       *command.DeleteReleasePlanHandler
	reopenPlanning   *command.ReopenReleasePlanPlanningHandler
	addJob           *command.AddReleasePlanJobHandler
	addJobsFromItems *command.AddReleasePlanJobsFromWorkItemsHandler
	updateJob        *command.UpdateReleasePlanJobHandler
	deleteJob        *command.DeleteReleasePlanJobHandler
	reorderJobs      *command.ReorderReleasePlanJobsHandler
	finishPlanning   *command.FinishReleasePlanPlanningHandler
	approvePlan      *command.ApproveReleasePlanHandler
	rejectPlan       *command.RejectReleasePlanHandler
	markJobDone      *command.MarkReleasePlanJobDoneHandler
	skipJob          *command.SkipReleasePlanJobHandler
	executePlan      *command.ExecuteReleasePlanHandler
	retryJob         *command.RetryReleasePlanJobHandler
	copyPlan         *command.CopyReleasePlanHandler
	saveTemplate     *command.SaveReleasePlanTemplateHandler
	createFromTpl    *command.CreateReleasePlanFromTemplateHandler
	listTemplates    *command.ListReleasePlanTemplatesHandler
	deleteTemplate   *command.DeleteReleasePlanTemplateHandler
	approveByToken   *command.ApproveReleasePlanByTokenHandler
	rejectByToken    *command.RejectReleasePlanByTokenHandler
	hookCallback     *command.ReleasePlanHookCallbackHandler
	listPlans        *query.ListReleasePlansHandler
	listWorkspace    *query.ListWorkspaceReleasePlansHandler
	getPlan          *query.GetReleasePlanHandlerWithSprint
	listVersions     *query.ListReleasePlanVersionsHandler
	getVersionDiff   *query.GetReleasePlanVersionDiffHandler
	blockingPlans    *query.ListBlockingReleasePlansHandler
}

func NewReleasePlanHandlers(
	createPlan *command.CreateReleasePlanHandler,
	updatePlan *command.UpdateReleasePlanHandler,
	cancelPlan *command.CancelReleasePlanHandler,
	deletePlan *command.DeleteReleasePlanHandler,
	reopenPlanning *command.ReopenReleasePlanPlanningHandler,
	addJob *command.AddReleasePlanJobHandler,
	addJobsFromItems *command.AddReleasePlanJobsFromWorkItemsHandler,
	updateJob *command.UpdateReleasePlanJobHandler,
	deleteJob *command.DeleteReleasePlanJobHandler,
	reorderJobs *command.ReorderReleasePlanJobsHandler,
	finishPlanning *command.FinishReleasePlanPlanningHandler,
	approvePlan *command.ApproveReleasePlanHandler,
	rejectPlan *command.RejectReleasePlanHandler,
	markJobDone *command.MarkReleasePlanJobDoneHandler,
	skipJob *command.SkipReleasePlanJobHandler,
	executePlan *command.ExecuteReleasePlanHandler,
	retryJob *command.RetryReleasePlanJobHandler,
	copyPlan *command.CopyReleasePlanHandler,
	saveTemplate *command.SaveReleasePlanTemplateHandler,
	createFromTpl *command.CreateReleasePlanFromTemplateHandler,
	listTemplates *command.ListReleasePlanTemplatesHandler,
	deleteTemplate *command.DeleteReleasePlanTemplateHandler,
	approveByToken *command.ApproveReleasePlanByTokenHandler,
	rejectByToken *command.RejectReleasePlanByTokenHandler,
	hookCallback *command.ReleasePlanHookCallbackHandler,
	listPlans *query.ListReleasePlansHandler,
	listWorkspace *query.ListWorkspaceReleasePlansHandler,
	getPlan *query.GetReleasePlanHandlerWithSprint,
	listVersions *query.ListReleasePlanVersionsHandler,
	getVersionDiff *query.GetReleasePlanVersionDiffHandler,
	blockingPlans *query.ListBlockingReleasePlansHandler,
) ReleasePlanHandlers {
	return ReleasePlanHandlers{
		createPlan: createPlan, updatePlan: updatePlan, cancelPlan: cancelPlan, deletePlan: deletePlan, reopenPlanning: reopenPlanning,
		addJob: addJob, addJobsFromItems: addJobsFromItems, updateJob: updateJob, deleteJob: deleteJob, reorderJobs: reorderJobs,
		finishPlanning: finishPlanning, approvePlan: approvePlan, rejectPlan: rejectPlan,
		markJobDone: markJobDone, skipJob: skipJob, executePlan: executePlan, retryJob: retryJob,
		copyPlan: copyPlan, saveTemplate: saveTemplate, createFromTpl: createFromTpl,
		listTemplates: listTemplates, deleteTemplate: deleteTemplate,
		approveByToken: approveByToken, rejectByToken: rejectByToken, hookCallback: hookCallback,
		listPlans: listPlans, listWorkspace: listWorkspace, getPlan: getPlan,
		listVersions: listVersions, getVersionDiff: getVersionDiff, blockingPlans: blockingPlans,
	}
}

func (h *Handler) ListReleasePlans(c *gin.Context) {
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	listQuery, err := parseReleasePlanListQueryFromRequest(c)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid list query", http.StatusBadRequest)
		return
	}
	page, err := h.releasePlans.listPlans.HandleFiltered(c.Request.Context(), productID, listQuery)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": page.Items, "has_more": page.HasMore})
}

func (h *Handler) ListWorkspaceReleasePlans(c *gin.Context) {
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	listQuery, err := parseReleasePlanListQueryFromRequest(c)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid list query", http.StatusBadRequest)
		return
	}
	items, err := h.releasePlans.listWorkspace.Handle(c.Request.Context(), wsID, listQuery)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func parseReleasePlanListQueryFromRequest(c *gin.Context) (query.ReleasePlanListQuery, error) {
	q, err := query.ParseReleasePlanListQuery(c.Query("status"), c.Query("from"), c.Query("to"))
	if err != nil {
		return q, err
	}
	if raw := c.Query("sprint_id"); raw != "" {
		parsed, err := kernel.ParseSprintID(raw)
		if err != nil {
			return q, err
		}
		q.SprintID = &parsed
	}
	q.Keyword = strings.TrimSpace(c.Query("q"))
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return q, fmt.Errorf("invalid limit")
		}
		q.Limit = min(n, query.MaxReleasePlanPageSize)
	}
	if raw := c.Query("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return q, fmt.Errorf("invalid offset")
		}
		q.Offset = n
	}
	return q, nil
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

func (h *Handler) ListReleasePlanVersions(c *gin.Context) {
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	items, err := h.releasePlans.listVersions.Handle(c.Request.Context(), productID, planID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetReleasePlanVersionDiff(c *gin.Context) {
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil || version <= 0 {
		httpx.Fail(c, "INVALID_REQUEST", "invalid version", http.StatusBadRequest)
		return
	}
	var compareVersion *int
	if raw := c.Query("compare"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			httpx.Fail(c, "INVALID_REQUEST", "invalid compare version", http.StatusBadRequest)
			return
		}
		compareVersion = &parsed
	}
	diff, err := h.releasePlans.getVersionDiff.Handle(c.Request.Context(), productID, planID, version, compareVersion)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, diff)
}

type approvalConfigRequest struct {
	Enabled     bool     `json:"enabled"`
	Provider    string   `json:"provider"`
	WebhookURL  string   `json:"webhook_url"`
	ApproverIDs []string `json:"approver_ids"`
}

type hookEventConfigRequest struct {
	Enabled     bool   `json:"enabled"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

type hookSettingsRequest struct {
	Enabled bool                                `json:"enabled"`
	Events  map[string]hookEventConfigRequest `json:"events"`
}

func parseHookSettingsRequest(req *hookSettingsRequest) *model.ReleasePlanHookSettings {
	if req == nil {
		return nil
	}
	out := &model.ReleasePlanHookSettings{Enabled: req.Enabled}
	if len(req.Events) > 0 {
		out.Events = make(map[model.ReleasePlanHookEvent]model.ReleasePlanHookEventConfig, len(req.Events))
		for key, cfg := range req.Events {
			out.Events[model.ReleasePlanHookEvent(key)] = model.ReleasePlanHookEventConfig{
				Enabled: cfg.Enabled, URL: cfg.URL, Description: cfg.Description,
			}
		}
	}
	return out
}

type createReleasePlanRequest struct {
	Name                string                 `json:"name" binding:"required"`
	Description         string                 `json:"description"`
	ManagerID           string                 `json:"manager_id"`
	ReleaseWindowStart  string                 `json:"release_window_start" binding:"required"`
	ReleaseWindowEnd    string                 `json:"release_window_end" binding:"required"`
	ApprovalEnabled     bool                   `json:"approval_enabled"`
	ApprovalConfig      *approvalConfigRequest `json:"approval_config"`
	ScheduleExecuteTime string                 `json:"schedule_execute_time"`
	SprintID            string                 `json:"sprint_id"`
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
		ApprovalConfig: parseApprovalConfigRequest(req.ApprovalConfig),
	}
	if req.ScheduleExecuteTime != "" {
		t, err := time.Parse(time.RFC3339, req.ScheduleExecuteTime)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid schedule_execute_time", http.StatusBadRequest)
			return
		}
		input.ScheduleExecuteTime = &t
	}
	if req.ManagerID != "" {
		mid, err := kernel.ParseMemberID(req.ManagerID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid manager_id", http.StatusBadRequest)
			return
		}
		input.ManagerID = mid
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
	TextFormat  string          `json:"text_format"`
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
		Name: req.Name, Type: jobType, TextContent: req.TextContent, TextFormat: req.TextFormat,
		SortOrder: req.SortOrder, WorkflowRef: req.WorkflowRef,
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
	var req struct {
		Remark string `json:"remark"`
	}
	_ = c.ShouldBindJSON(&req)
	plan, err := h.releasePlans.markJobDone.Handle(c.Request.Context(), productID, planID, jobID, memberID, req.Remark)
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

type updateReleasePlanRequest struct {
	Name                *string                `json:"name"`
	Description         *string                `json:"description"`
	ManagerID           *string                `json:"manager_id"`
	ApproverID          *string                `json:"approver_id"`
	ReleaseWindowStart  *string                `json:"release_window_start"`
	ReleaseWindowEnd    *string                `json:"release_window_end"`
	ApprovalEnabled     *bool                  `json:"approval_enabled"`
	ApprovalConfig      *approvalConfigRequest `json:"approval_config"`
	HookSettings        *hookSettingsRequest   `json:"hook_settings"`
	Parameters          *map[string]string     `json:"parameters"`
	ScheduleExecuteTime *string                `json:"schedule_execute_time"`
	ClearScheduleTime   bool                   `json:"clear_schedule_time"`
	SprintID            *string                `json:"sprint_id"`
	ClearSprintID       bool                   `json:"clear_sprint_id"`
}

func (h *Handler) UpdateReleasePlan(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	var req updateReleasePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	input := command.UpdateReleasePlanInput{
		Name: req.Name, Description: req.Description, ApprovalEnabled: req.ApprovalEnabled,
		ApprovalConfig: parseApprovalConfigRequest(req.ApprovalConfig),
		HookSettings:   parseHookSettingsRequest(req.HookSettings),
		Parameters:     req.Parameters,
		ClearSprintID: req.ClearSprintID, ClearScheduleTime: req.ClearScheduleTime,
	}
	if req.ScheduleExecuteTime != nil && *req.ScheduleExecuteTime != "" {
		t, err := time.Parse(time.RFC3339, *req.ScheduleExecuteTime)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid schedule_execute_time", http.StatusBadRequest)
			return
		}
		input.ScheduleExecuteTime = &t
	}
	if req.ManagerID != nil {
		mid, err := kernel.ParseMemberID(*req.ManagerID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid manager_id", http.StatusBadRequest)
			return
		}
		input.ManagerID = &mid
	}
	if req.ApproverID != nil {
		aid, err := kernel.ParseMemberID(*req.ApproverID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid approver_id", http.StatusBadRequest)
			return
		}
		input.ApproverID = &aid
	}
	if req.ReleaseWindowStart != nil {
		t, err := time.Parse(time.RFC3339, *req.ReleaseWindowStart)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid release_window_start", http.StatusBadRequest)
			return
		}
		input.ReleaseWindowStart = &t
	}
	if req.ReleaseWindowEnd != nil {
		t, err := time.Parse(time.RFC3339, *req.ReleaseWindowEnd)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid release_window_end", http.StatusBadRequest)
			return
		}
		input.ReleaseWindowEnd = &t
	}
	if req.SprintID != nil && *req.SprintID != "" {
		sid, err := kernel.ParseSprintID(*req.SprintID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid sprint_id", http.StatusBadRequest)
			return
		}
		input.SprintID = &sid
	}
	plan, err := h.releasePlans.updatePlan.Handle(c.Request.Context(), productID, planID, memberID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func (h *Handler) CancelReleasePlan(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	plan, err := h.releasePlans.cancelPlan.Handle(c.Request.Context(), productID, planID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func (h *Handler) DeleteReleasePlan(c *gin.Context) {
	_, _, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	if err := h.releasePlans.deletePlan.Handle(c.Request.Context(), productID, planID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"ok": true})
}

func (h *Handler) ReopenReleasePlanPlanning(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	plan, err := h.releasePlans.reopenPlanning.Handle(c.Request.Context(), productID, planID, memberID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func (h *Handler) ReorderReleasePlanJobs(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	var req struct {
		JobIDs []string `json:"job_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	ids := make([]kernel.ReleasePlanJobID, 0, len(req.JobIDs))
	for _, raw := range req.JobIDs {
		id, err := kernel.ParseReleasePlanJobID(raw)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid job id", http.StatusBadRequest)
			return
		}
		ids = append(ids, id)
	}
	plan, err := h.releasePlans.reorderJobs.Handle(c.Request.Context(), productID, planID, memberID, command.ReorderReleasePlanJobsInput{JobIDs: ids})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

type updateReleasePlanJobRequest struct {
	Name        *string         `json:"name"`
	OwnerID     *string         `json:"owner_id"`
	TextContent *string         `json:"text_content"`
	TextFormat  *string         `json:"text_format"`
	SortOrder   *int            `json:"sort_order"`
	WorkflowRef json.RawMessage `json:"workflow_ref"`
}

func (h *Handler) UpdateReleasePlanJob(c *gin.Context) {
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
	var req updateReleasePlanJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	input := command.UpdateReleasePlanJobInput{
		Name: req.Name, TextContent: req.TextContent, TextFormat: req.TextFormat, SortOrder: req.SortOrder,
	}
	if req.OwnerID != nil {
		oid, err := kernel.ParseMemberID(*req.OwnerID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid owner_id", http.StatusBadRequest)
			return
		}
		input.OwnerID = &oid
	}
	if len(req.WorkflowRef) > 0 {
		input.HasWorkflow = true
		input.WorkflowRef = req.WorkflowRef
	}
	job, err := h.releasePlans.updateJob.Handle(c.Request.Context(), productID, planID, jobID, memberID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": job.ID.String(), "status": job.Status})
}

func (h *Handler) DeleteReleasePlanJob(c *gin.Context) {
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
	if err := h.releasePlans.deleteJob.Handle(c.Request.Context(), productID, planID, jobID, memberID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"ok": true})
}

func (h *Handler) AddReleasePlanJobsFromWorkItems(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	var req struct {
		WorkItemIDs []string `json:"work_item_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	ids := make([]kernel.WorkItemID, 0, len(req.WorkItemIDs))
	for _, raw := range req.WorkItemIDs {
		id, err := kernel.ParseWorkItemID(raw)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid work_item_id", http.StatusBadRequest)
			return
		}
		ids = append(ids, id)
	}
	plan, err := h.releasePlans.addJobsFromItems.Handle(c.Request.Context(), productID, planID, memberID, command.AddReleasePlanJobsFromWorkItemsInput{WorkItemIDs: ids})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func (h *Handler) CopyReleasePlan(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&req)
	plan, err := h.releasePlans.copyPlan.Handle(c.Request.Context(), productID, planID, memberID, command.CopyReleasePlanInput{Name: req.Name})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func (h *Handler) SaveReleasePlanTemplate(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	_ = c.ShouldBindJSON(&req)
	tpl, err := h.releasePlans.saveTemplate.Handle(c.Request.Context(), wsID, productID, planID, memberID, command.SaveReleasePlanTemplateInput{
		Name: req.Name, Description: req.Description,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": tpl.ID.String(), "name": tpl.Name})
}

func (h *Handler) ListReleasePlanTemplates(c *gin.Context) {
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	items, err := h.releasePlans.listTemplates.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, gin.H{
			"id": item.ID.String(), "name": item.Name, "description": item.Description,
			"job_count": len(item.Snapshot.Jobs), "created_at": item.CreatedAt.Format(time.RFC3339),
		})
	}
	httpx.OK(c, gin.H{"items": out})
}

func (h *Handler) CreateReleasePlanFromTemplate(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	templateID, err := kernel.ParseReleasePlanTemplateID(c.Param("templateId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid template id", http.StatusBadRequest)
		return
	}
	var req struct {
		ProductID           string `json:"product_id" binding:"required"`
		Name                string `json:"name"`
		Description         string `json:"description"`
		ManagerID           string `json:"manager_id"`
		ReleaseWindowStart  string `json:"release_window_start" binding:"required"`
		ReleaseWindowEnd    string `json:"release_window_end" binding:"required"`
		ScheduleExecuteTime string `json:"schedule_execute_time"`
		SprintID            string `json:"sprint_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	productID, err := kernel.ParseProductID(req.ProductID)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
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
	input := command.CreateReleasePlanFromTemplateInput{
		ProductID: productID, Name: req.Name, Description: req.Description,
		ReleaseWindowStart: start, ReleaseWindowEnd: end,
	}
	if req.ManagerID != "" {
		mid, err := kernel.ParseMemberID(req.ManagerID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid manager_id", http.StatusBadRequest)
			return
		}
		input.ManagerID = mid
	}
	if req.ScheduleExecuteTime != "" {
		t, err := time.Parse(time.RFC3339, req.ScheduleExecuteTime)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid schedule_execute_time", http.StatusBadRequest)
			return
		}
		input.ScheduleExecuteTime = &t
	}
	if req.SprintID != "" {
		sid, err := kernel.ParseSprintID(req.SprintID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid sprint_id", http.StatusBadRequest)
			return
		}
		input.SprintID = &sid
	}
	plan, err := h.releasePlans.createFromTpl.Handle(c.Request.Context(), wsID, templateID, memberID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status, "product_id": plan.ProductID.String()})
}

func (h *Handler) DeleteReleasePlanTemplate(c *gin.Context) {
	wsID, _, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	templateID, err := kernel.ParseReleasePlanTemplateID(c.Param("templateId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid template id", http.StatusBadRequest)
		return
	}
	if err := h.releasePlans.deleteTemplate.Handle(c.Request.Context(), wsID, templateID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"ok": true})
}

func (h *Handler) PublicApproveReleasePlanByToken(c *gin.Context) {
	token := c.Param("token")
	var req struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req)
	plan, err := h.releasePlans.approveByToken.Handle(c.Request.Context(), token, command.ApprovalByTokenInput{Note: req.Note})
	if err != nil {
		if c.Request.Method == http.MethodGet {
			c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(command.FormatApprovalHTML("审批失败", err.Error())))
			return
		}
		httpx.HandleError(c, err)
		return
	}
	if c.Request.Method == http.MethodGet {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(command.FormatApprovalHTML("审批成功", "发布计划「"+plan.Name+"」已批准")))
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func (h *Handler) PublicRejectReleasePlanByToken(c *gin.Context) {
	token := c.Param("token")
	var req struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Note == "" {
		req.Note = c.Query("note")
	}
	plan, err := h.releasePlans.rejectByToken.Handle(c.Request.Context(), token, command.ApprovalByTokenInput{Note: req.Note})
	if err != nil {
		if c.Request.Method == http.MethodGet {
			c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(command.FormatApprovalHTML("操作失败", err.Error())))
			return
		}
		httpx.HandleError(c, err)
		return
	}
	if c.Request.Method == http.MethodGet {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(command.FormatApprovalHTML("已拒绝", "发布计划「"+plan.Name+"」已驳回")))
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func (h *Handler) PublicPassReleasePlanHook(c *gin.Context) {
	h.handlePublicReleasePlanHook(c, true)
}

func (h *Handler) PublicFailReleasePlanHook(c *gin.Context) {
	h.handlePublicReleasePlanHook(c, false)
}

func (h *Handler) handlePublicReleasePlanHook(c *gin.Context, pass bool) {
	token := c.Param("token")
	var req struct {
		Message string `json:"message"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Message == "" {
		req.Message = c.Query("message")
	}
	plan, err := h.releasePlans.hookCallback.Handle(c.Request.Context(), token, command.ReleasePlanHookCallbackInput{
		Pass: pass, Message: req.Message,
	})
	title := "外部校验通过"
	body := "发布计划「" + plan.Name + "」外部校验已通过"
	if !pass {
		title = "外部校验失败"
		body = "发布计划「" + plan.Name + "」外部校验未通过"
		if req.Message != "" {
			body += "：" + req.Message
		}
	}
	if err != nil {
		if c.Request.Method == http.MethodGet {
			c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(command.FormatApprovalHTML("操作失败", err.Error())))
			return
		}
		httpx.HandleError(c, err)
		return
	}
	if c.Request.Method == http.MethodGet {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(command.FormatApprovalHTML(title, body)))
		return
	}
	httpx.OK(c, gin.H{"id": plan.ID.String(), "status": plan.Status})
}

func parseApprovalConfigRequest(req *approvalConfigRequest) *model.ApprovalConfig {
	if req == nil {
		return nil
	}
	return &model.ApprovalConfig{
		Enabled: req.Enabled, Provider: model.ApprovalProvider(req.Provider),
		WebhookURL: req.WebhookURL, ApproverIDs: req.ApproverIDs,
	}
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

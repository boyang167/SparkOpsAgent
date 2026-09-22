package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type WorkflowHandlers struct {
	CreateFromTemplate *command.CreateWorkflowFromTemplateHandler
	CreateBlank        *command.CreateBlankWorkflowHandler
	Update             *command.UpdateWorkflowHandler
	Delete             *command.DeleteWorkflowHandler
	GetWorkflow        *query.GetProductWorkflowHandler
	StartRun           *command.StartWorkflowRunHandler
	ListWorkflows      *query.ListProductWorkflowsHandler
	ListRuns           *query.ListWorkflowRunsHandler
	GetRun             *query.GetWorkflowRunHandler
	UpdateRunRemark    *command.UpdateWorkflowRunRemarkHandler
	CancelRun          *command.CancelWorkflowRunHandler
	PauseRun           *command.PauseWorkflowRunHandler
	ApproveJob         *command.ApproveWorkflowJobHandler
	RejectJob          *command.RejectWorkflowJobHandler
	ContinueStage      *command.ContinueManualStageHandler
}

type createFromTemplateRequest struct {
	TemplateID string            `json:"template_id" binding:"required"`
	Name       string            `json:"name"`
	KeyVals    map[string]string `json:"key_vals"`
	ProjectIDs []string          `json:"project_ids" binding:"required"`
}

type workflowStageDTO struct {
	Name     string           `json:"name"`
	Parallel bool             `json:"parallel"`
	ExecMode string           `json:"exec_mode"`
	Jobs     []workflowJobDTO `json:"jobs"`
}

type workflowJobDTO struct {
	Name string         `json:"name"`
	Type string         `json:"type"`
	Spec map[string]any `json:"spec"`
}

type createBlankWorkflowRequest struct {
	Name        string             `json:"name" binding:"required"`
	Key         string             `json:"key"`
	Description string             `json:"description"`
	Stages      []workflowStageDTO `json:"stages"`
	Activate    bool               `json:"activate"`
}

type updateWorkflowRequest struct {
	Name          *string                    `json:"name"`
	Description   *string                    `json:"description"`
	Status        *string                    `json:"status"`
	Stages        []workflowStageDTO         `json:"stages"`
	Notifications *[]workflowNotificationDTO `json:"notifications"`
	Variables     *[]workflowVariableDTO     `json:"variables"`
	Triggers      *[]workflowTriggerDTO      `json:"triggers"`
	Advanced      *workflowAdvancedDTO       `json:"advanced"`
}

type startWorkflowRunRequest struct {
	Input map[string]any `json:"input"`
}

type workflowNotificationDTO struct {
	ID         string   `json:"id"`
	Enabled    *bool    `json:"enabled"`
	Events     []string `json:"events"`
	Channel    string   `json:"channel"`
	WebhookURL string   `json:"webhook_url"`
	Targets    string   `json:"targets"`
}

type workflowVariableDTO struct {
	ID           string `json:"id"`
	Key          string `json:"key"`
	Value        string `json:"value"`
	Description  string `json:"description"`
	RuntimeInput bool   `json:"runtime_input"`
	Secret       bool   `json:"secret"`
	VaultPath    string `json:"vault_path"`
}

type workflowTriggerDTO struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Enabled      *bool    `json:"enabled"`
	ProjectID    string   `json:"project_id"`
	Events       []string `json:"events"`
	BranchFilter string   `json:"branch_filter"`
	Token        string   `json:"token"`
	Schedule     string   `json:"schedule"`
}

type workflowAdvancedDTO struct {
	Concurrency    int  `json:"concurrency"`
	TimeoutMinutes int  `json:"timeout_minutes"`
	Debug          bool `json:"debug"`
}

func dtoToNotifications(items []workflowNotificationDTO) []model.WorkflowNotification {
	out := make([]model.WorkflowNotification, 0, len(items))
	for _, item := range items {
		out = append(out, model.WorkflowNotification{
			ID: item.ID, Enabled: item.Enabled, Events: item.Events, Channel: item.Channel, WebhookURL: item.WebhookURL, Targets: item.Targets,
		})
	}
	return out
}

func dtoToVariables(items []workflowVariableDTO) []model.WorkflowVariable {
	out := make([]model.WorkflowVariable, 0, len(items))
	for _, item := range items {
		out = append(out, model.WorkflowVariable{
			ID: item.ID, Key: item.Key, Value: item.Value, Description: item.Description,
			RuntimeInput: item.RuntimeInput, Secret: item.Secret, VaultPath: item.VaultPath,
		})
	}
	return out
}

func dtoToTriggers(items []workflowTriggerDTO) []model.WorkflowTrigger {
	out := make([]model.WorkflowTrigger, 0, len(items))
	for _, item := range items {
		out = append(out, model.WorkflowTrigger{
			ID: item.ID, Type: item.Type, Enabled: item.Enabled, ProjectID: item.ProjectID,
			Events: item.Events, BranchFilter: item.BranchFilter, Token: item.Token, Schedule: item.Schedule,
		})
	}
	return out
}

func workflowPayload(wf *model.ProductWorkflow) gin.H {
	variables := model.MaskSecretVariables(wf.Variables)
	if variables == nil {
		variables = []model.WorkflowVariable{}
	}
	triggers := wf.Triggers
	if triggers == nil {
		triggers = []model.WorkflowTrigger{}
	}
	notes := wf.Notifications
	if notes == nil {
		notes = []model.WorkflowNotification{}
	}
	advanced := wf.Advanced
	return gin.H{
		"id": wf.ID.String(), "name": wf.Name, "key": wf.Key, "status": wf.Status,
		"stages": wf.Stages, "notifications": notes, "variables": variables, "triggers": triggers, "advanced": advanced,
	}
}

func dtoToStages(items []workflowStageDTO) []model.WorkflowStage {
	out := make([]model.WorkflowStage, 0, len(items))
	for _, stage := range items {
		jobs := make([]model.WorkflowJob, 0, len(stage.Jobs))
		for _, job := range stage.Jobs {
			spec := job.Spec
			if spec == nil {
				spec = map[string]any{}
			}
			jobs = append(jobs, model.WorkflowJob{Name: job.Name, Type: job.Type, Spec: spec})
		}
		out = append(out, model.WorkflowStage{
			Name: stage.Name, Parallel: stage.Parallel, ExecMode: model.NormalizeStageExecMode(stage.ExecMode), Jobs: jobs,
		})
	}
	return out
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

func (h *Handler) GetProductWorkflow(c *gin.Context) {
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
	item, err := h.workflows.GetWorkflow.Handle(c.Request.Context(), wsID, productID, workflowID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) CreateBlankWorkflow(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	var req createBlankWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	wf, err := h.workflows.CreateBlank.Handle(c.Request.Context(), wsID, productID, memberID, command.CreateBlankWorkflowInput{
		Name: req.Name, Key: req.Key, Description: req.Description, Stages: dtoToStages(req.Stages), Activate: req.Activate,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, workflowPayload(wf))
}

func (h *Handler) UpdateProductWorkflow(c *gin.Context) {
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
	var req updateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	input := command.UpdateWorkflowInput{Name: req.Name, Description: req.Description, Status: req.Status}
	if req.Stages != nil {
		input.Stages = dtoToStages(req.Stages)
	}
	if req.Notifications != nil {
		notes := dtoToNotifications(*req.Notifications)
		input.Notifications = &notes
	}
	if req.Variables != nil {
		variables := dtoToVariables(*req.Variables)
		input.Variables = &variables
	}
	if req.Triggers != nil {
		triggers := dtoToTriggers(*req.Triggers)
		input.Triggers = &triggers
	}
	if req.Advanced != nil {
		advanced := model.WorkflowAdvanced{Concurrency: req.Advanced.Concurrency, TimeoutMinutes: req.Advanced.TimeoutMinutes, Debug: req.Advanced.Debug}
		input.Advanced = &advanced
	}
	wf, err := h.workflows.Update.Handle(c.Request.Context(), wsID, productID, workflowID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, workflowPayload(wf))
}

func (h *Handler) DeleteProductWorkflow(c *gin.Context) {
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
	if err := h.workflows.Delete.Handle(c.Request.Context(), wsID, productID, workflowID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"ok": true})
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
	wsID, memberID, ok := memberAndWorkspace(c)
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
	var req startWorkflowRunRequest
	_ = c.ShouldBindJSON(&req)
	run, err := h.workflows.StartRun.Handle(c.Request.Context(), wsID, productID, workflowID, memberID, req.Input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": run.ID.String(), "status": run.Status, "stage_runs": run.StageRuns})
}

type updateWorkflowRunRemarkRequest struct {
	Remark string `json:"remark"`
}

func (h *Handler) UpdateWorkflowRunRemark(c *gin.Context) {
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
	var req updateWorkflowRunRemarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.workflows.UpdateRunRemark.Handle(c.Request.Context(), wsID, productID, runID, req.Remark); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"ok": true})
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

type cancelWorkflowRunRequest struct {
	Message string `json:"message"`
}

type rejectWorkflowJobRequest struct {
	Message string `json:"message"`
}

func (h *Handler) CancelWorkflowRun(c *gin.Context) {
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
	var req cancelWorkflowRunRequest
	_ = c.ShouldBindJSON(&req)
	if err := h.workflows.CancelRun.Handle(c.Request.Context(), wsID, productID, runID, req.Message); err != nil {
		httpx.HandleError(c, err)
		return
	}
	item, err := h.workflows.GetRun.Handle(c.Request.Context(), wsID, productID, runID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
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

func (h *Handler) PauseWorkflowRun(c *gin.Context) {
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
	if err := h.workflows.PauseRun.Handle(c.Request.Context(), wsID, productID, runID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	item, err := h.workflows.GetRun.Handle(c.Request.Context(), wsID, productID, runID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) ContinueWorkflowStage(c *gin.Context) {
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
	if err := h.workflows.ContinueStage.Handle(c.Request.Context(), wsID, productID, runID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"ok": true})
}

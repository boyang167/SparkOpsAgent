package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SprintHandlers struct {
	getOrSeedProcess   *command.GetOrSeedSprintProcessHandler
	upsertProcess      *command.UpsertSprintProcessHandler
	listProcesses      *command.ListSprintProcessesHandler
	createProcess      *command.CreateSprintProcessHandler
	updateProcessByID  *command.UpdateSprintProcessByIDHandler
	deleteProcess      *command.DeleteSprintProcessHandler
	importWsTemplate   *command.ImportWorkspaceTemplateToProductHandler
	listWsTemplates    *command.ListWorkspaceSprintTemplatesHandler
	createWsTemplate   *command.CreateWorkspaceSprintTemplateHandler
	deleteWsTemplate   *command.DeleteWorkspaceSprintTemplateHandler
	createSprint     *command.CreateSprintHandler
	updateSprint     *command.UpdateSprintHandler
	startSprint      *command.StartSprintHandler
	completeSprint   *command.CompleteSprintHandler
	archiveSprint    *command.ArchiveSprintHandler
	createWorkItem   *command.CreateWorkItemHandler
	updateWorkItem   *command.UpdateWorkItemHandler
	moveWorkItem     *command.MoveWorkItemHandler
	deleteWorkItem   *command.DeleteWorkItemHandler
	executeWorkItem  *command.ExecuteWorkItemHandler
	listSprints      *query.ListSprintsHandler
	getSprint        *query.GetSprintHandler
	getBoard         *query.GetSprintBoardHandler
	getWorkItem      *query.GetWorkItemHandler
	listExecutions   *query.ListWorkItemExecutionsHandler
	importPm         *command.ImportPmSprintHandler
	linkPm           *command.LinkPmIssueHandler
	getMetrics       *query.GetSprintMetricsHandler
	listActive       *query.ListActiveSprintsHandler
}

func NewSprintHandlers(
	getOrSeedProcess *command.GetOrSeedSprintProcessHandler,
	upsertProcess *command.UpsertSprintProcessHandler,
	listProcesses *command.ListSprintProcessesHandler,
	createProcess *command.CreateSprintProcessHandler,
	updateProcessByID *command.UpdateSprintProcessByIDHandler,
	deleteProcess *command.DeleteSprintProcessHandler,
	importWsTemplate *command.ImportWorkspaceTemplateToProductHandler,
	listWsTemplates *command.ListWorkspaceSprintTemplatesHandler,
	createWsTemplate *command.CreateWorkspaceSprintTemplateHandler,
	deleteWsTemplate *command.DeleteWorkspaceSprintTemplateHandler,
	createSprint *command.CreateSprintHandler,
	updateSprint *command.UpdateSprintHandler,
	startSprint *command.StartSprintHandler,
	completeSprint *command.CompleteSprintHandler,
	archiveSprint *command.ArchiveSprintHandler,
	createWorkItem *command.CreateWorkItemHandler,
	updateWorkItem *command.UpdateWorkItemHandler,
	moveWorkItem *command.MoveWorkItemHandler,
	deleteWorkItem *command.DeleteWorkItemHandler,
	executeWorkItem *command.ExecuteWorkItemHandler,
	listSprints *query.ListSprintsHandler,
	getSprint *query.GetSprintHandler,
	getBoard *query.GetSprintBoardHandler,
	getWorkItem *query.GetWorkItemHandler,
	listExecutions *query.ListWorkItemExecutionsHandler,
	importPm *command.ImportPmSprintHandler,
	linkPm *command.LinkPmIssueHandler,
	getMetrics *query.GetSprintMetricsHandler,
	listActive *query.ListActiveSprintsHandler,
) SprintHandlers {
	return SprintHandlers{
		getOrSeedProcess: getOrSeedProcess, upsertProcess: upsertProcess,
		createSprint: createSprint, updateSprint: updateSprint,
		startSprint: startSprint, completeSprint: completeSprint, archiveSprint: archiveSprint,
		createWorkItem: createWorkItem, updateWorkItem: updateWorkItem,
		moveWorkItem: moveWorkItem, deleteWorkItem: deleteWorkItem, executeWorkItem: executeWorkItem,
		listSprints: listSprints, getSprint: getSprint, getBoard: getBoard, getWorkItem: getWorkItem,
		listExecutions: listExecutions,
		importPm: importPm, linkPm: linkPm,
		getMetrics: getMetrics, listActive: listActive,
	}
}

func (h *Handler) ListActiveSprints(c *gin.Context) {
	if _, _, ok := memberAndWorkspace(c); !ok {
		return
	}
	wsID, err := kernel.ParseWorkspaceID(c.Param("wsId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workspace id", http.StatusBadRequest)
		return
	}
	items, err := h.sprints.listActive.Handle(c.Request.Context(), wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetSprintMetrics(c *gin.Context) {
	productID, sprintID, ok := parseProductSprintIDs(c)
	if !ok {
		return
	}
	metrics, err := h.sprints.getMetrics.Handle(c.Request.Context(), productID, sprintID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, metrics)
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
	process, err := h.sprints.getOrSeedProcess.Handle(c.Request.Context(), productID, memberID)
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
	process, err := h.sprints.upsertProcess.Handle(c.Request.Context(), productID, memberID, command.UpsertSprintProcessInput{
		Name: req.Name, Stages: req.Stages,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, sprintProcessVM(process))
}

func (h *Handler) ListSprints(c *gin.Context) {
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	items, err := h.sprints.listSprints.Handle(c.Request.Context(), productID, c.Query("status"))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

type createSprintRequest struct {
	Name         string  `json:"name" binding:"required"`
	Goal         string  `json:"goal"`
	PlannedStart *string `json:"planned_start"`
	PlannedEnd   *string `json:"planned_end"`
	OwnerID      *string `json:"owner_id"`
	ProcessID    *string `json:"process_id"`
}

func (h *Handler) CreateSprint(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	var req createSprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	input := command.CreateSprintInput{Name: req.Name, Goal: req.Goal}
	if req.PlannedStart != nil {
		t, err := time.Parse(time.RFC3339, *req.PlannedStart)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid planned_start", http.StatusBadRequest)
			return
		}
		input.PlannedStart = &t
	}
	if req.PlannedEnd != nil {
		t, err := time.Parse(time.RFC3339, *req.PlannedEnd)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid planned_end", http.StatusBadRequest)
			return
		}
		input.PlannedEnd = &t
	}
	if req.OwnerID != nil {
		oid, err := kernel.ParseMemberID(*req.OwnerID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid owner_id", http.StatusBadRequest)
			return
		}
		input.OwnerID = &oid
	}
	if req.ProcessID != nil {
		pid, err := kernel.ParseSprintProcessID(*req.ProcessID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid process_id", http.StatusBadRequest)
			return
		}
		input.ProcessID = &pid
	}
	sprint, err := h.sprints.createSprint.Handle(c.Request.Context(), productID, memberID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": sprint.ID.String(), "status": sprint.Status})
}

func (h *Handler) GetSprint(c *gin.Context) {
	productID, sprintID, ok := parseProductSprintIDs(c)
	if !ok {
		return
	}
	item, err := h.sprints.getSprint.Handle(c.Request.Context(), productID, sprintID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

type updateSprintRequest struct {
	Name              *string             `json:"name"`
	Goal              *string             `json:"goal"`
	PlannedStart      *string             `json:"planned_start"`
	PlannedEnd        *string             `json:"planned_end"`
	OwnerID           *string             `json:"owner_id"`
	Stages            []model.SprintStage `json:"stages"`
	StageMigrations   map[string]string   `json:"stage_migrations"`
}

func (h *Handler) UpdateSprint(c *gin.Context) {
	productID, sprintID, ok := parseProductSprintIDs(c)
	if !ok {
		return
	}
	var req updateSprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	input := command.UpdateSprintInput{Name: req.Name, Goal: req.Goal, Stages: req.Stages, StageMigrations: req.StageMigrations}
	if req.PlannedStart != nil {
		t, err := time.Parse(time.RFC3339, *req.PlannedStart)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid planned_start", http.StatusBadRequest)
			return
		}
		input.PlannedStart = &t
	}
	if req.PlannedEnd != nil {
		t, err := time.Parse(time.RFC3339, *req.PlannedEnd)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid planned_end", http.StatusBadRequest)
			return
		}
		input.PlannedEnd = &t
	}
	if req.OwnerID != nil {
		oid, err := kernel.ParseMemberID(*req.OwnerID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid owner_id", http.StatusBadRequest)
			return
		}
		input.OwnerID = &oid
	}
	sprint, err := h.sprints.updateSprint.Handle(c.Request.Context(), productID, sprintID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": sprint.ID.String(), "status": sprint.Status})
}

func (h *Handler) GetSprintBoard(c *gin.Context) {
	productID, sprintID, ok := parseProductSprintIDs(c)
	if !ok {
		return
	}
	board, err := h.sprints.getBoard.Handle(c.Request.Context(), productID, sprintID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, board)
}

func (h *Handler) StartSprint(c *gin.Context) {
	productID, sprintID, ok := parseProductSprintIDs(c)
	if !ok {
		return
	}
	sprint, err := h.sprints.startSprint.Handle(c.Request.Context(), productID, sprintID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": sprint.ID.String(), "status": sprint.Status})
}

func (h *Handler) CompleteSprint(c *gin.Context) {
	productID, sprintID, ok := parseProductSprintIDs(c)
	if !ok {
		return
	}
	sprint, err := h.sprints.completeSprint.Handle(c.Request.Context(), productID, sprintID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": sprint.ID.String(), "status": sprint.Status})
}

func (h *Handler) ArchiveSprint(c *gin.Context) {
	productID, sprintID, ok := parseProductSprintIDs(c)
	if !ok {
		return
	}
	sprint, err := h.sprints.archiveSprint.Handle(c.Request.Context(), productID, sprintID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": sprint.ID.String(), "status": sprint.Status})
}

type createWorkItemRequest struct {
	StageID     string             `json:"stage_id"`
	Type        model.WorkItemType `json:"type"`
	Title       string             `json:"title" binding:"required"`
	Description string             `json:"description"`
	Priority    model.WorkItemPriority `json:"priority"`
	AssigneeID  *string            `json:"assignee_id"`
	ProjectIDs  []string           `json:"project_ids"`
	Labels      []string           `json:"labels"`
	SortOrder   int                `json:"sort_order"`
}

func (h *Handler) CreateWorkItem(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, sprintID, ok := parseProductSprintIDs(c)
	if !ok {
		return
	}
	var req createWorkItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	input := command.CreateWorkItemInput{
		StageID: req.StageID, Type: req.Type, Title: req.Title, Description: req.Description,
		Priority: req.Priority, ProjectIDs: req.ProjectIDs, Labels: req.Labels, SortOrder: req.SortOrder,
	}
	if req.AssigneeID != nil {
		aid, err := kernel.ParseMemberID(*req.AssigneeID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid assignee_id", http.StatusBadRequest)
			return
		}
		input.AssigneeID = &aid
	}
	item, err := h.sprints.createWorkItem.Handle(c.Request.Context(), productID, sprintID, memberID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": item.ID.String()})
}

func (h *Handler) GetWorkItem(c *gin.Context) {
	productID, workItemID, ok := parseProductWorkItemIDs(c)
	if !ok {
		return
	}
	item, err := h.sprints.getWorkItem.Handle(c.Request.Context(), productID, workItemID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

type updateWorkItemRequest struct {
	Title         *string                 `json:"title"`
	Description   *string                 `json:"description"`
	Type          *model.WorkItemType     `json:"type"`
	Priority      *model.WorkItemPriority `json:"priority"`
	Status        *model.WorkItemStatus   `json:"status"`
	AssigneeID    *string                 `json:"assignee_id"`
	ProjectIDs    []string                `json:"project_ids"`
	Labels        []string                `json:"labels"`
	BlockedReason *string                 `json:"blocked_reason"`
	ClearAssignee bool                    `json:"clear_assignee"`
}

func (h *Handler) UpdateWorkItem(c *gin.Context) {
	productID, workItemID, ok := parseProductWorkItemIDs(c)
	if !ok {
		return
	}
	var req updateWorkItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	input := command.UpdateWorkItemInput{
		Title: req.Title, Description: req.Description, Type: req.Type,
		Priority: req.Priority, Status: req.Status, ProjectIDs: req.ProjectIDs,
		Labels: req.Labels, BlockedReason: req.BlockedReason,
	}
	input.ClearAssignee = req.ClearAssignee
	if req.AssigneeID != nil {
		aid, err := kernel.ParseMemberID(*req.AssigneeID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid assignee_id", http.StatusBadRequest)
			return
		}
		input.AssigneeID = &aid
	}
	item, err := h.sprints.updateWorkItem.Handle(c.Request.Context(), productID, workItemID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": item.ID.String(), "version": item.Version})
}

type moveWorkItemRequest struct {
	StageID   string `json:"stage_id" binding:"required"`
	SortOrder int    `json:"sort_order"`
	Version   int    `json:"version" binding:"required"`
}

func (h *Handler) MoveWorkItem(c *gin.Context) {
	productID, workItemID, ok := parseProductWorkItemIDs(c)
	if !ok {
		return
	}
	var req moveWorkItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.sprints.moveWorkItem.Handle(c.Request.Context(), productID, workItemID, command.MoveWorkItemInput{
		StageID: req.StageID, SortOrder: req.SortOrder, Version: req.Version,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": item.ID.String(), "stage_id": item.StageID, "version": item.Version})
}

func (h *Handler) DeleteWorkItem(c *gin.Context) {
	productID, workItemID, ok := parseProductWorkItemIDs(c)
	if !ok {
		return
	}
	if err := h.sprints.deleteWorkItem.Handle(c.Request.Context(), productID, workItemID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

type executeWorkItemRequest struct {
	WorkflowID string         `json:"workflow_id"`
	StageID    string         `json:"stage_id"`
	Input      map[string]any `json:"input"`
}

func (h *Handler) ExecuteWorkItem(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, workItemID, ok := parseProductWorkItemIDs(c)
	if !ok {
		return
	}
	var req executeWorkItemRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
			return
		}
	}
	result, err := h.sprints.executeWorkItem.Handle(c.Request.Context(), productID, workItemID, memberID, command.ExecuteWorkItemInput{
		WorkflowID: req.WorkflowID, StageID: req.StageID, Input: req.Input,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"execution_id":    result.ExecutionID.String(),
		"workflow_run_id": result.WorkflowRunID.String(),
	})
}

type importJiraRequest struct {
	JiraSprintID  string   `json:"jira_sprint_id" binding:"required"`
	JiraBoardID   string   `json:"jira_board_id"`
	TargetStageID string   `json:"target_stage_id"`
	IssueTypes    []string `json:"issue_types"`
	SyncMode      string   `json:"sync_mode"`
}

func (h *Handler) ImportJiraSprint(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, sprintID, ok := parseProductSprintIDs(c)
	if !ok {
		return
	}
	var req importJiraRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.sprints.importPm.Handle(c.Request.Context(), productID, sprintID, memberID, command.ImportPmSprintInput{
		Provider: port.PmProviderJira, IterationID: req.JiraSprintID, TargetStageID: req.TargetStageID,
		IssueTypes: req.IssueTypes, SyncMode: req.SyncMode,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	ids := make([]string, 0, len(result.WorkItemIDs))
	for _, id := range result.WorkItemIDs {
		ids = append(ids, id.String())
	}
	httpx.OK(c, gin.H{
		"sprint_id":       result.SprintID.String(),
		"imported_count":  result.ImportedCount,
		"skipped_count":   result.SkippedCount,
		"work_item_ids":   ids,
	})
}

type linkJiraRequest struct {
	IssueKey string `json:"issue_key" binding:"required"`
	SyncMode string `json:"sync_mode"`
}

type importPmRequest struct {
	Provider      string   `json:"provider" binding:"required"`
	PmWorkspaceID string   `json:"pm_workspace_id"`
	IterationID   string   `json:"iteration_id" binding:"required"`
	TargetStageID string   `json:"target_stage_id"`
	IssueTypes    []string `json:"issue_types"`
	SyncMode      string   `json:"sync_mode"`
}

func (h *Handler) ImportPmSprint(c *gin.Context) {
	_, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, sprintID, ok := parseProductSprintIDs(c)
	if !ok {
		return
	}
	var req importPmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.sprints.importPm.Handle(c.Request.Context(), productID, sprintID, memberID, command.ImportPmSprintInput{
		Provider: req.Provider, PmWorkspaceID: req.PmWorkspaceID, IterationID: req.IterationID,
		TargetStageID: req.TargetStageID, IssueTypes: req.IssueTypes, SyncMode: req.SyncMode,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	ids := make([]string, 0, len(result.WorkItemIDs))
	for _, id := range result.WorkItemIDs {
		ids = append(ids, id.String())
	}
	httpx.OK(c, gin.H{
		"sprint_id": result.SprintID.String(), "imported_count": result.ImportedCount,
		"skipped_count": result.SkippedCount, "work_item_ids": ids,
	})
}

type linkPmRequest struct {
	Provider      string `json:"provider" binding:"required"`
	PmWorkspaceID string `json:"pm_workspace_id"`
	IssueKey      string `json:"issue_key" binding:"required"`
	SyncMode      string `json:"sync_mode"`
}

func (h *Handler) LinkPmIssue(c *gin.Context) {
	productID, workItemID, ok := parseProductWorkItemIDs(c)
	if !ok {
		return
	}
	var req linkPmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.sprints.linkPm.Handle(c.Request.Context(), productID, workItemID, command.LinkPmIssueInput{
		Provider: req.Provider, PmWorkspaceID: req.PmWorkspaceID, IssueKey: req.IssueKey, SyncMode: req.SyncMode,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": item.ID.String(), "external": item.ExternalRef})
}

func (h *Handler) LinkJiraIssue(c *gin.Context) {
	productID, workItemID, ok := parseProductWorkItemIDs(c)
	if !ok {
		return
	}
	var req linkJiraRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	item, err := h.sprints.linkPm.Handle(c.Request.Context(), productID, workItemID, command.LinkPmIssueInput{
		Provider: port.PmProviderJira, IssueKey: req.IssueKey, SyncMode: req.SyncMode,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": item.ID.String(), "external": item.ExternalRef})
}

func (h *Handler) ListWorkItemExecutions(c *gin.Context) {
	productID, workItemID, ok := parseProductWorkItemIDs(c)
	if !ok {
		return
	}
	items, err := h.sprints.listExecutions.Handle(c.Request.Context(), productID, workItemID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func parseProductSprintIDs(c *gin.Context) (kernel.ProductID, kernel.SprintID, bool) {
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return kernel.ProductID{}, kernel.SprintID{}, false
	}
	sprintID, err := kernel.ParseSprintID(c.Param("sprintId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid sprint id", http.StatusBadRequest)
		return kernel.ProductID{}, kernel.SprintID{}, false
	}
	return productID, sprintID, true
}

func parseProductWorkItemIDs(c *gin.Context) (kernel.ProductID, kernel.WorkItemID, bool) {
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return kernel.ProductID{}, kernel.WorkItemID{}, false
	}
	workItemID, err := kernel.ParseWorkItemID(c.Param("workItemId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid work item id", http.StatusBadRequest)
		return kernel.ProductID{}, kernel.WorkItemID{}, false
	}
	return productID, workItemID, true
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

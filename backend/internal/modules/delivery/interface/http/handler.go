package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	trigger  *command.TriggerBuildHandler
	abort    *command.AbortBuildHandler
	rollback *command.RollbackBuildHandler
	list     *query.ListBuildsHandler
	get      *query.GetBuildHandler
	stream   *query.StreamBuildLogsHandler
	branches    *query.ListGitBranchesHandler
	listCron    *query.ListCronJobsHandler
	createCron  *command.CreateCronJobHandler
	deleteCron  *command.DeleteCronJobHandler
}

func NewHandler(
	trigger *command.TriggerBuildHandler,
	abort *command.AbortBuildHandler,
	rollback *command.RollbackBuildHandler,
	list *query.ListBuildsHandler,
	get *query.GetBuildHandler,
	stream *query.StreamBuildLogsHandler,
	branches *query.ListGitBranchesHandler,
	listCron *query.ListCronJobsHandler,
	createCron *command.CreateCronJobHandler,
	deleteCron *command.DeleteCronJobHandler,
) *Handler {
	return &Handler{
		trigger: trigger, abort: abort, rollback: rollback,
		list: list, get: get, stream: stream, branches: branches,
		listCron: listCron, createCron: createCron, deleteCron: deleteCron,
	}
}

type triggerBuildRequest struct {
	Branch string `json:"branch"`
}

func (h *Handler) TriggerBuild(c *gin.Context) {
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
	var req triggerBuildRequest
	_ = c.ShouldBindJSON(&req)
	build, err := h.trigger.Handle(c.Request.Context(), wsID, memberID, projectID, req.Branch)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": build.ID.String(), "project_id": build.ProjectID.String(),
		"status": string(build.Status), "branch": build.Branch,
		"build_number": build.BuildNumber, "jenkins_job": build.JenkinsJob,
	})
}

func (h *Handler) ListBuilds(c *gin.Context) {
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

func (h *Handler) GetBuild(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	buildID, err := kernel.ParseBuildID(c.Param("buildId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid build id", http.StatusBadRequest)
		return
	}
	item, err := h.get.Handle(c.Request.Context(), wsID, buildID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) StreamBuildLogs(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	buildID, err := kernel.ParseBuildID(c.Param("buildId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid build id", http.StatusBadRequest)
		return
	}
	ch, initial, done, err := h.stream.Subscribe(c.Request.Context(), wsID, buildID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.StreamSSE(c, ch, initial, done)
}

func (h *Handler) AbortBuild(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	buildID, err := kernel.ParseBuildID(c.Param("buildId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid build id", http.StatusBadRequest)
		return
	}
	if err := h.abort.Handle(c.Request.Context(), wsID, buildID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"aborted": true})
}

func (h *Handler) RollbackBuild(c *gin.Context) {
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
	buildID, err := kernel.ParseBuildID(c.Param("buildId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid build id", http.StatusBadRequest)
		return
	}
	build, err := h.rollback.Handle(c.Request.Context(), wsID, memberID, buildID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": build.ID.String(), "status": string(build.Status),
		"build_number": build.BuildNumber, "rollback_of_build_id": build.RollbackOfBuildID,
	})
}

func (h *Handler) ListGitBranches(c *gin.Context) {
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
	items, err := h.branches.Handle(c.Request.Context(), wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

type createCronJobRequest struct {
	Name     string `json:"name" binding:"required"`
	Schedule string `json:"schedule" binding:"required"`
	Branch   string `json:"branch"`
}

func (h *Handler) ListCronJobs(c *gin.Context) {
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
	items, err := h.listCron.Handle(c.Request.Context(), wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) CreateCronJob(c *gin.Context) {
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
	var req createCronJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	job, err := h.createCron.Handle(c.Request.Context(), wsID, projectID, req.Name, req.Schedule, req.Branch)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"id": job.ID.String(), "project_id": job.ProjectID.String(),
		"name": job.Name, "schedule": job.Schedule, "branch": job.Branch, "enabled": job.Enabled,
	})
}

func (h *Handler) DeleteCronJob(c *gin.Context) {
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
	cronJobID, err := kernel.ParseCronJobID(c.Param("cronJobId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid cronjob id", http.StatusBadRequest)
		return
	}
	if err := h.deleteCron.Handle(c.Request.Context(), wsID, projectID, cronJobID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

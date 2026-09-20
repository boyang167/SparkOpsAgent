package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	create      *command.CreateDeploymentHandler
	rollback    *command.RollbackDeploymentHandler
	abort       *command.AbortDeploymentHandler
	list        *query.ListDeploymentsHandler
	get         *query.GetDeploymentHandler
	options     *query.DeploymentOptionsHandler
	preview     *query.RollbackPreviewHandler
	syncPreview *query.SyncPreviewHandler
	runPreview  *query.RunPreviewHandler
	stream      *query.StreamDeploymentHandler
}

func NewHandler(
	create *command.CreateDeploymentHandler,
	rollback *command.RollbackDeploymentHandler,
	abort *command.AbortDeploymentHandler,
	list *query.ListDeploymentsHandler,
	get *query.GetDeploymentHandler,
	options *query.DeploymentOptionsHandler,
	preview *query.RollbackPreviewHandler,
	syncPreview *query.SyncPreviewHandler,
	runPreview *query.RunPreviewHandler,
	stream *query.StreamDeploymentHandler,
) *Handler {
	return &Handler{
		create: create, rollback: rollback, abort: abort, list: list, get: get,
		options: options, preview: preview, syncPreview: syncPreview, runPreview: runPreview, stream: stream,
	}
}

func (h *Handler) Create(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	projectID, ok := projectID(c)
	if !ok {
		return
	}
	var input command.CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	run, err := h.create.Handle(c.Request.Context(), wsID, memberID, projectID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, httpx.Envelope{
		Code: 0, Message: "ok", Data: query.ToDeploymentView(run), TraceID: httpx.GetTraceID(c),
	})
}

func (h *Handler) List(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	projectID, ok := projectID(c)
	if !ok {
		return
	}
	page, err := h.list.Handle(c.Request.Context(), wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, page)
}

func (h *Handler) Options(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	projectID, ok := projectID(c)
	if !ok {
		return
	}
	options, err := h.options.Handle(c.Request.Context(), wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, options)
}

func (h *Handler) Get(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	runID, ok := deploymentRunID(c)
	if !ok {
		return
	}
	item, err := h.get.Handle(c.Request.Context(), wsID, runID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) Stream(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	runID, ok := deploymentRunID(c)
	if !ok {
		return
	}
	ch, initial, done, unsubscribe, err := h.stream.Subscribe(c.Request.Context(), wsID, runID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	defer unsubscribe()
	httpx.StreamSSE(c, ch, initial, done)
}

func (h *Handler) Rollback(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	projectID, ok := projectID(c)
	if !ok {
		return
	}
	var input command.RollbackInput
	if err := c.ShouldBindJSON(&input); err != nil && c.Request.ContentLength > 0 {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	run, err := h.rollback.Handle(c.Request.Context(), wsID, memberID, projectID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, httpx.Envelope{
		Code: 0, Message: "ok", Data: query.ToDeploymentView(run), TraceID: httpx.GetTraceID(c),
	})
}

func (h *Handler) RollbackPreview(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	projectID, ok := projectID(c)
	if !ok {
		return
	}
	preview, err := h.preview.Handle(c.Request.Context(), wsID, projectID, c.Query("target_run_id"))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, preview)
}

func (h *Handler) SyncPreview(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	projectID, ok := projectID(c)
	if !ok {
		return
	}
	preview, err := h.syncPreview.Handle(c.Request.Context(), wsID, projectID, c.Query("image_tag"))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, preview)
}

func (h *Handler) RunPreview(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	projectID, ok := projectID(c)
	if !ok {
		return
	}
	preview, err := h.runPreview.Handle(
		c.Request.Context(),
		wsID,
		projectID,
		c.Query("environment_id"),
		c.Query("image_tag"),
	)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, preview)
}

func (h *Handler) Abort(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	runID, ok := deploymentRunID(c)
	if !ok {
		return
	}
	run, err := h.abort.Handle(c.Request.Context(), wsID, runID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, query.ToDeploymentView(run))
}

func workspaceID(c *gin.Context) (kernel.WorkspaceID, bool) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
	}
	return wsID, ok
}

func projectID(c *gin.Context) (kernel.ProjectID, bool) {
	id, err := kernel.ParseProjectID(c.Param("projectId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
		return kernel.ProjectID{}, false
	}
	return id, true
}

func deploymentRunID(c *gin.Context) (kernel.DeploymentRunID, bool) {
	id, err := kernel.ParseDeploymentRunID(c.Param("runId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid deployment run id", http.StatusBadRequest)
		return kernel.DeploymentRunID{}, false
	}
	return id, true
}

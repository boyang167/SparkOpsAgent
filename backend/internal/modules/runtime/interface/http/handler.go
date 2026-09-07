package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	listPods    *query.ListPodsHandler
	getPod      *query.GetPodHandler
	podLogs     *query.GetPodLogsHandler
	terminal    *query.StreamPodTerminalHandler
	overview    *query.GetProjectRuntimeOverviewHandler
	restartPod  *command.RestartPodHandler
}

func NewHandler(
	listPods *query.ListPodsHandler,
	getPod *query.GetPodHandler,
	podLogs *query.GetPodLogsHandler,
	terminal *query.StreamPodTerminalHandler,
	overview *query.GetProjectRuntimeOverviewHandler,
	restartPod *command.RestartPodHandler,
) *Handler {
	return &Handler{
		listPods: listPods, getPod: getPod, podLogs: podLogs, terminal: terminal,
		overview: overview, restartPod: restartPod,
	}
}

func (h *Handler) GetProjectRuntimeOverview(c *gin.Context) {
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
	item, err := h.overview.Handle(c.Request.Context(), wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) ListPods(c *gin.Context) {
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
	items, err := h.listPods.Handle(c.Request.Context(), wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetPod(c *gin.Context) {
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
	podID, err := kernel.ParsePodID(c.Param("podId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid pod id", http.StatusBadRequest)
		return
	}
	item, err := h.getPod.Handle(c.Request.Context(), wsID, projectID, podID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) GetPodLogs(c *gin.Context) {
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
	podID, err := kernel.ParsePodID(c.Param("podId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid pod id", http.StatusBadRequest)
		return
	}
	logs, err := h.podLogs.Handle(c.Request.Context(), wsID, projectID, podID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"logs": logs})
}

func (h *Handler) StreamPodTerminal(c *gin.Context) {
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
	podID, err := kernel.ParsePodID(c.Param("podId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid pod id", http.StatusBadRequest)
		return
	}
	lines, err := h.terminal.Lines(c.Request.Context(), wsID, projectID, podID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	ch := make(chan string)
	go func() {
		defer close(ch)
		for _, line := range lines {
			select {
			case <-c.Request.Context().Done():
				return
			case ch <- line + "\n":
				time.Sleep(250 * time.Millisecond)
			}
		}
	}()
	httpx.StreamSSE(c, ch, "", false)
}

func (h *Handler) RestartPod(c *gin.Context) {
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
	podID, err := kernel.ParsePodID(c.Param("podId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid pod id", http.StatusBadRequest)
		return
	}
	err = h.restartPod.Handle(c.Request.Context(), wsID, projectID, podID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"restarted": true})
}

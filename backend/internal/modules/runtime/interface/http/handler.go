package http

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/application/query"
	runtimek8s "github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/infrastructure/k8s"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	listPods       *query.ListPodsHandler
	getPod         *query.GetPodHandler
	podLogs        *query.GetPodLogsHandler
	terminal       *query.StreamPodTerminalHandler
	overview       *query.GetProjectRuntimeOverviewHandler
	restartPod     *command.RestartPodHandler
	runtimeConfig  *command.RuntimeConfigHandler
}

func NewHandler(
	listPods *query.ListPodsHandler,
	getPod *query.GetPodHandler,
	podLogs *query.GetPodLogsHandler,
	terminal *query.StreamPodTerminalHandler,
	overview *query.GetProjectRuntimeOverviewHandler,
	restartPod *command.RestartPodHandler,
	runtimeConfig *command.RuntimeConfigHandler,
) *Handler {
	return &Handler{
		listPods: listPods, getPod: getPod, podLogs: podLogs, terminal: terminal,
		overview: overview, restartPod: restartPod, runtimeConfig: runtimeConfig,
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

func (h *Handler) PutLogCollection(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		var body runtimek8s.LogCollectionInput
		if err := c.ShouldBindJSON(&body); err != nil {
			return err
		}
		return h.runtimeConfig.UpsertLogCollection(ctx, wsID, projectID, body)
	})
}

func (h *Handler) PutHealthProbe(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		var body runtimek8s.HealthProbeInput
		if err := c.ShouldBindJSON(&body); err != nil {
			return err
		}
		return h.runtimeConfig.UpsertHealthProbe(ctx, wsID, projectID, body)
	})
}

func (h *Handler) PutServicePorts(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		var body struct {
			Ports []runtimek8s.ServicePortInput `json:"ports"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			return err
		}
		return h.runtimeConfig.UpdateServicePorts(ctx, wsID, projectID, body.Ports)
	})
}

func (h *Handler) CreateRuntimeService(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		return h.runtimeConfig.CreateService(ctx, wsID, projectID)
	})
}

func (h *Handler) CreateRuntimeIngress(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		var body runtimek8s.IngressInput
		_ = c.ShouldBindJSON(&body)
		return h.runtimeConfig.CreateIngress(ctx, wsID, projectID, body)
	})
}

func (h *Handler) handleRuntimeConfig(
	c *gin.Context,
	run func(context.Context, kernel.WorkspaceID, kernel.ProjectID) error,
) {
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
	if h.runtimeConfig == nil {
		httpx.Fail(c, "RUNTIME_NOT_CONFIGURED", "runtime configurator unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := run(c.Request.Context(), wsID, projectID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"ok": true})
}

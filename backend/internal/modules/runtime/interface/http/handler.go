package http

import (
	"context"
	"fmt"
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
	deploymentYAML *query.GetDeploymentYAMLHandler
	podYAML        *query.GetPodYAMLHandler
	restartPod     *command.RestartPodHandler
	runtimeConfig  *command.RuntimeConfigHandler
}

func NewHandler(
	listPods *query.ListPodsHandler,
	getPod *query.GetPodHandler,
	podLogs *query.GetPodLogsHandler,
	terminal *query.StreamPodTerminalHandler,
	overview *query.GetProjectRuntimeOverviewHandler,
	deploymentYAML *query.GetDeploymentYAMLHandler,
	podYAML *query.GetPodYAMLHandler,
	restartPod *command.RestartPodHandler,
	runtimeConfig *command.RuntimeConfigHandler,
) *Handler {
	return &Handler{
		listPods: listPods, getPod: getPod, podLogs: podLogs, terminal: terminal,
		overview: overview, deploymentYAML: deploymentYAML, podYAML: podYAML,
		restartPod: restartPod, runtimeConfig: runtimeConfig,
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

func (h *Handler) GetDeploymentYAML(c *gin.Context) {
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
	if h.deploymentYAML == nil {
		httpx.Fail(c, "K8S_RESOURCE_NOT_FOUND", "deployment yaml unavailable", http.StatusNotFound)
		return
	}
	yamlText, err := h.deploymentYAML.Handle(c.Request.Context(), wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"yaml": yamlText})
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

func (h *Handler) GetPodYAML(c *gin.Context) {
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
	if h.podYAML == nil {
		httpx.Fail(c, "K8S_POD_NOT_FOUND", "pod yaml unavailable", http.StatusNotFound)
		return
	}
	yamlText, err := h.podYAML.Handle(c.Request.Context(), wsID, projectID, podID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"yaml": yamlText})
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

func (h *Handler) PutDeploymentResources(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		var body runtimek8s.DeploymentResourcesInput
		if err := c.ShouldBindJSON(&body); err != nil {
			return err
		}
		return h.runtimeConfig.UpdateDeploymentResources(ctx, wsID, projectID, body)
	})
}

func (h *Handler) ListConfigMaps(c *gin.Context) {
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
	items, err := h.runtimeConfig.ListConfigMaps(c.Request.Context(), wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) CreateConfigMap(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		var body runtimek8s.CreateConfigMapInput
		_ = c.ShouldBindJSON(&body)
		return h.runtimeConfig.CreateConfigMap(ctx, wsID, projectID, body)
	})
}

func (h *Handler) UpsertConfigMapEntry(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		var body runtimek8s.UpsertConfigMapEntryInput
		if err := c.ShouldBindJSON(&body); err != nil {
			return err
		}
		return h.runtimeConfig.UpsertConfigMapEntry(ctx, wsID, projectID, c.Param("name"), body)
	})
}

func (h *Handler) ReplaceConfigMapData(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		var body runtimek8s.ReplaceConfigMapDataInput
		if err := c.ShouldBindJSON(&body); err != nil {
			return err
		}
		return h.runtimeConfig.ReplaceConfigMapData(ctx, wsID, projectID, c.Param("name"), body)
	})
}

func (h *Handler) DeleteConfigMapEntry(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		key := c.Query("key")
		if key == "" {
			return fmt.Errorf("key is required")
		}
		return h.runtimeConfig.DeleteConfigMapEntry(ctx, wsID, projectID, c.Param("name"), key)
	})
}

func (h *Handler) ListPVCs(c *gin.Context) {
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
	items, err := h.runtimeConfig.ListPVCs(c.Request.Context(), wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) ListStorageClasses(c *gin.Context) {
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
	items, err := h.runtimeConfig.ListStorageClasses(c.Request.Context(), wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) GetStorageOverview(c *gin.Context) {
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
	overview, err := h.runtimeConfig.GetStorageOverview(c.Request.Context(), wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, overview)
}

func (h *Handler) CreatePVC(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		var body runtimek8s.CreatePVCInput
		_ = c.ShouldBindJSON(&body)
		return h.runtimeConfig.CreatePVC(ctx, wsID, projectID, body)
	})
}

func (h *Handler) BindPVC(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		var body runtimek8s.BindPVCInput
		if err := c.ShouldBindJSON(&body); err != nil {
			return err
		}
		return h.runtimeConfig.BindPVC(ctx, wsID, projectID, c.Param("name"), body)
	})
}

func (h *Handler) UnbindPVC(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		return h.runtimeConfig.UnbindPVC(ctx, wsID, projectID, c.Param("name"))
	})
}

func (h *Handler) DeletePVC(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		return h.runtimeConfig.DeletePVC(ctx, wsID, projectID, c.Param("name"))
	})
}

func (h *Handler) PutDeploymentScale(c *gin.Context) {
	h.handleRuntimeConfig(c, func(ctx context.Context, wsID kernel.WorkspaceID, projectID kernel.ProjectID) error {
		var body struct {
			Replicas int32 `json:"replicas"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			return err
		}
		return h.runtimeConfig.ScaleDeployment(ctx, wsID, projectID, body.Replicas)
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

package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/runtime/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ResourcesHandler struct {
	k8s *query.K8sResources
}

func NewResourcesHandler(k8s *query.K8sResources) *ResourcesHandler {
	return &ResourcesHandler{k8s: k8s}
}

func (h *ResourcesHandler) ListDeployments(c *gin.Context) {
	h.listProject(c, func(wsID kernel.WorkspaceID, projectID kernel.ProjectID) (any, error) {
		items, err := h.k8s.ListDeployments(c.Request.Context(), wsID, projectID)
		return gin.H{"items": items}, err
	})
}

func (h *ResourcesHandler) GetDeployment(c *gin.Context) {
	h.getProject(c, func(wsID kernel.WorkspaceID, projectID kernel.ProjectID) (any, error) {
		return h.k8s.GetDeployment(c.Request.Context(), wsID, projectID, c.Param("name"))
	})
}

func (h *ResourcesHandler) ScaleDeployment(c *gin.Context) {
	wsID, projectID, ok := h.parseProject(c)
	if !ok {
		return
	}
	var body struct {
		Replicas int32 `json:"replicas"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.k8s.ScaleDeployment(c.Request.Context(), wsID, projectID, c.Param("name"), body.Replicas); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"scaled": true})
}

func (h *ResourcesHandler) ListIngresses(c *gin.Context) {
	h.listProject(c, func(wsID kernel.WorkspaceID, projectID kernel.ProjectID) (any, error) {
		items, err := h.k8s.ListIngresses(c.Request.Context(), wsID, projectID)
		return gin.H{"items": items}, err
	})
}

func (h *ResourcesHandler) GetIngress(c *gin.Context) {
	h.getProject(c, func(wsID kernel.WorkspaceID, projectID kernel.ProjectID) (any, error) {
		return h.k8s.GetIngress(c.Request.Context(), wsID, projectID, c.Param("name"))
	})
}

func (h *ResourcesHandler) ListConfigMaps(c *gin.Context) {
	h.listProject(c, func(wsID kernel.WorkspaceID, projectID kernel.ProjectID) (any, error) {
		items, err := h.k8s.ListConfigMaps(c.Request.Context(), wsID, projectID)
		return gin.H{"items": items}, err
	})
}

func (h *ResourcesHandler) GetConfigMap(c *gin.Context) {
	h.getProject(c, func(wsID kernel.WorkspaceID, projectID kernel.ProjectID) (any, error) {
		return h.k8s.GetConfigMap(c.Request.Context(), wsID, projectID, c.Param("name"))
	})
}

func (h *ResourcesHandler) ListServices(c *gin.Context) {
	h.listProject(c, func(wsID kernel.WorkspaceID, projectID kernel.ProjectID) (any, error) {
		items, err := h.k8s.ListServices(c.Request.Context(), wsID, projectID)
		return gin.H{"items": items}, err
	})
}

func (h *ResourcesHandler) GetService(c *gin.Context) {
	h.getProject(c, func(wsID kernel.WorkspaceID, projectID kernel.ProjectID) (any, error) {
		return h.k8s.GetService(c.Request.Context(), wsID, projectID, c.Param("name"))
	})
}

func (h *ResourcesHandler) ListPVCs(c *gin.Context) {
	h.listProject(c, func(wsID kernel.WorkspaceID, projectID kernel.ProjectID) (any, error) {
		items, err := h.k8s.ListPVCs(c.Request.Context(), wsID, projectID)
		return gin.H{"items": items}, err
	})
}

func (h *ResourcesHandler) GetPVC(c *gin.Context) {
	h.getProject(c, func(wsID kernel.WorkspaceID, projectID kernel.ProjectID) (any, error) {
		return h.k8s.GetPVC(c.Request.Context(), wsID, projectID, c.Param("name"))
	})
}

func (h *ResourcesHandler) ListStorageClasses(c *gin.Context) {
	items, err := h.k8s.ListStorageClasses(c.Request.Context())
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *ResourcesHandler) GetStorageClass(c *gin.Context) {
	item, err := h.k8s.GetStorageClass(c.Request.Context(), c.Param("name"))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *ResourcesHandler) ListVirtualServices(c *gin.Context) {
	h.listProject(c, func(wsID kernel.WorkspaceID, projectID kernel.ProjectID) (any, error) {
		items, err := h.k8s.ListVirtualServices(c.Request.Context(), wsID, projectID)
		return gin.H{"items": items}, err
	})
}

func (h *ResourcesHandler) GetVirtualService(c *gin.Context) {
	h.getProject(c, func(wsID kernel.WorkspaceID, projectID kernel.ProjectID) (any, error) {
		return h.k8s.GetVirtualService(c.Request.Context(), wsID, projectID, c.Param("name"))
	})
}

type projectFn func(kernel.WorkspaceID, kernel.ProjectID) (any, error)

func (h *ResourcesHandler) listProject(c *gin.Context, fn projectFn) {
	wsID, projectID, ok := h.parseProject(c)
	if !ok {
		return
	}
	data, err := fn(wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, data)
}

func (h *ResourcesHandler) getProject(c *gin.Context, fn projectFn) {
	wsID, projectID, ok := h.parseProject(c)
	if !ok {
		return
	}
	data, err := fn(wsID, projectID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, data)
}

func (h *ResourcesHandler) parseProject(c *gin.Context) (kernel.WorkspaceID, kernel.ProjectID, bool) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return kernel.WorkspaceID{}, kernel.ProjectID{}, false
	}
	projectID, err := kernel.ParseProjectID(c.Param("projectId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
		return kernel.WorkspaceID{}, kernel.ProjectID{}, false
	}
	return wsID, projectID, true
}

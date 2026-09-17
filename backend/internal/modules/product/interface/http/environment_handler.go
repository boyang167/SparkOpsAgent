package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type EnvironmentHandlers struct {
	CreateEnvironment *command.CreateEnvironmentHandler
	UpdateEnvironment *command.UpdateEnvironmentHandler
	DeleteEnvironment *command.DeleteEnvironmentHandler
	ListEnvironments  *query.ListEnvironmentsHandler
	GetStatus         *query.GetEnvironmentStatusHandler
}

type createEnvironmentRequest struct {
	Name       string            `json:"name" binding:"required"`
	EnvType    string            `json:"env_type"`
	ClusterID  string            `json:"cluster_id" binding:"required"`
	Namespace  string            `json:"namespace" binding:"required"`
	Variables  map[string]string `json:"variables"`
	ProjectIDs []string          `json:"project_ids"`
}

type updateEnvironmentRequest struct {
	Name      *string            `json:"name"`
	EnvType   *string            `json:"env_type"`
	ClusterID *string            `json:"cluster_id"`
	Namespace *string            `json:"namespace"`
	Variables map[string]string  `json:"variables"`
	Status    *string            `json:"status"`
}

func (h *Handler) ListEnvironments(c *gin.Context) {
	wsID, _, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	items, err := h.environments.ListEnvironments.Handle(c.Request.Context(), wsID, productID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) CreateEnvironment(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	var req createEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	clusterID, err := kernel.ParseClusterInfoID(req.ClusterID)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid cluster id", http.StatusBadRequest)
		return
	}
	projectIDs := make([]kernel.ProjectID, 0, len(req.ProjectIDs))
	for _, raw := range req.ProjectIDs {
		pid, parseErr := kernel.ParseProjectID(raw)
		if parseErr != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid project id", http.StatusBadRequest)
			return
		}
		projectIDs = append(projectIDs, pid)
	}
	env, err := h.environments.CreateEnvironment.Handle(c.Request.Context(), wsID, productID, memberID, command.CreateEnvironmentInput{
		Name: req.Name, EnvType: model.EnvironmentType(req.EnvType), ClusterID: clusterID,
		Namespace: req.Namespace, Variables: req.Variables, ProjectIDs: projectIDs,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": env.ID.String(), "name": env.Name, "status": env.Status})
}

func (h *Handler) UpdateEnvironment(c *gin.Context) {
	wsID, _, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	envID, err := kernel.ParseEnvironmentID(c.Param("envId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid environment id", http.StatusBadRequest)
		return
	}
	var req updateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	input := command.UpdateEnvironmentInput{Variables: req.Variables}
	if req.Name != nil {
		input.Name = req.Name
	}
	if req.EnvType != nil {
		v := model.EnvironmentType(*req.EnvType)
		input.EnvType = &v
	}
	if req.ClusterID != nil {
		clusterID, parseErr := kernel.ParseClusterInfoID(*req.ClusterID)
		if parseErr != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid cluster id", http.StatusBadRequest)
			return
		}
		input.ClusterID = &clusterID
	}
	if req.Namespace != nil {
		input.Namespace = req.Namespace
	}
	if req.Status != nil {
		v := model.EnvironmentStatus(*req.Status)
		input.Status = &v
	}
	env, err := h.environments.UpdateEnvironment.Handle(c.Request.Context(), wsID, productID, envID, input)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": env.ID.String(), "name": env.Name, "status": env.Status})
}

func (h *Handler) DeleteEnvironment(c *gin.Context) {
	wsID, _, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	envID, err := kernel.ParseEnvironmentID(c.Param("envId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid environment id", http.StatusBadRequest)
		return
	}
	if err := h.environments.DeleteEnvironment.Handle(c.Request.Context(), wsID, productID, envID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *Handler) GetEnvironmentStatus(c *gin.Context) {
	wsID, _, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	productID, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return
	}
	envID, err := kernel.ParseEnvironmentID(c.Param("envId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid environment id", http.StatusBadRequest)
		return
	}
	item, err := h.environments.GetStatus.Handle(c.Request.Context(), wsID, productID, envID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type EnvironmentSyncHandler struct {
	preview       *query.CreateEnvironmentSyncPreviewHandler
	sourceResources *query.ListEnvironmentSyncSourceResourcesHandler
}

func NewEnvironmentSyncHandler(
	preview *query.CreateEnvironmentSyncPreviewHandler,
	sourceResources *query.ListEnvironmentSyncSourceResourcesHandler,
) *EnvironmentSyncHandler {
	return &EnvironmentSyncHandler{preview: preview, sourceResources: sourceResources}
}

type environmentSyncProjectRequest struct {
	ProjectID         string            `json:"project_id" binding:"required"`
	SyncImage         bool              `json:"sync_image"`
	SyncYAML          bool              `json:"sync_yaml"`
	SyncResourceKinds []string          `json:"sync_resource_kinds"`
	ResourceOverrides map[string]string `json:"resource_overrides"`
}

type createEnvironmentSyncPreviewRequest struct {
	WorkflowID          string                          `json:"workflow_id" binding:"required"`
	JobName             string                          `json:"job_name" binding:"required"`
	SourceEnvironmentID string                          `json:"source_environment_id" binding:"required"`
	TargetEnvironmentID string                          `json:"target_environment_id" binding:"required"`
	Projects            []environmentSyncProjectRequest `json:"projects" binding:"required,min=1,dive"`
}

func (h *EnvironmentSyncHandler) CreatePreview(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	productID, ok := productIDFromParam(c)
	if !ok {
		return
	}
	var req createEnvironmentSyncPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	workflowID, err := kernel.ParseProductWorkflowID(req.WorkflowID)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid workflow_id", http.StatusBadRequest)
		return
	}
	sourceID, err := kernel.ParseEnvironmentID(req.SourceEnvironmentID)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid source_environment_id", http.StatusBadRequest)
		return
	}
	targetID, err := kernel.ParseEnvironmentID(req.TargetEnvironmentID)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid target_environment_id", http.StatusBadRequest)
		return
	}
	projects, ok := parseEnvironmentSyncProjectOptions(req.Projects)
	if !ok {
		httpx.Fail(c, "INVALID_REQUEST", "invalid project_id", http.StatusBadRequest)
		return
	}
	snapshot, err := h.preview.Handle(c.Request.Context(), wsID, productID, memberID, query.CreateEnvironmentSyncPreviewInput{
		WorkflowID:          workflowID,
		JobName:             req.JobName,
		SourceEnvironmentID: sourceID,
		TargetEnvironmentID: targetID,
		Projects:            projects,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, httpx.Envelope{
		Code: 0, Message: "ok", Data: query.ToEnvironmentSyncPreviewView(snapshot), TraceID: httpx.GetTraceID(c),
	})
}

type listEnvironmentSyncSourceResourcesRequest struct {
	SourceEnvironmentID string                          `json:"source_environment_id" binding:"required"`
	Projects            []environmentSyncProjectRequest `json:"projects" binding:"required,min=1,dive"`
}

func (h *EnvironmentSyncHandler) ListSourceResources(c *gin.Context) {
	wsID, ok := workspaceID(c)
	if !ok {
		return
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	_ = memberID
	productID, ok := productIDFromParam(c)
	if !ok {
		return
	}
	var req listEnvironmentSyncSourceResourcesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	sourceID, err := kernel.ParseEnvironmentID(req.SourceEnvironmentID)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid source_environment_id", http.StatusBadRequest)
		return
	}
	projects, ok := parseEnvironmentSyncProjectOptions(req.Projects)
	if !ok {
		return
	}
	items, err := h.sourceResources.Handle(c.Request.Context(), wsID, productID, query.ListEnvironmentSyncSourceResourcesInput{
		SourceEnvironmentID: sourceID,
		Projects:            projects,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, httpx.Envelope{
		Code: 0, Message: "ok", Data: map[string]any{"projects": items}, TraceID: httpx.GetTraceID(c),
	})
}

func parseEnvironmentSyncProjectOptions(items []environmentSyncProjectRequest) ([]model.EnvironmentSyncProjectOption, bool) {
	projects := make([]model.EnvironmentSyncProjectOption, 0, len(items))
	for _, item := range items {
		projectID, parseErr := kernel.ParseProjectID(item.ProjectID)
		if parseErr != nil {
			return nil, false
		}
		projects = append(projects, model.EnvironmentSyncProjectOption{
			ProjectID:         projectID,
			SyncImage:         item.SyncImage,
			SyncYAML:          item.SyncYAML,
			SyncResourceKinds: append([]string(nil), item.SyncResourceKinds...),
			ResourceOverrides: item.ResourceOverrides,
		})
	}
	return projects, true
}

func productIDFromParam(c *gin.Context) (kernel.ProductID, bool) {
	id, err := kernel.ParseProductID(c.Param("productId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid product id", http.StatusBadRequest)
		return kernel.ProductID{}, false
	}
	return id, true
}

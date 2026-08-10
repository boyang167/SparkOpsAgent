package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	create  *command.CreateAgentHandler
	update  *command.UpdateAgentHandler
	archive *command.ArchiveAgentHandler
	addVer  *command.AddVersionHandler
	publish *command.PublishAgentHandler
	list    *query.ListAgentsHandler
	get     *query.GetAgentHandler
}

func NewHandler(
	create *command.CreateAgentHandler,
	update *command.UpdateAgentHandler,
	archive *command.ArchiveAgentHandler,
	addVer *command.AddVersionHandler,
	publish *command.PublishAgentHandler,
	list *query.ListAgentsHandler,
	get *query.GetAgentHandler,
) *Handler {
	return &Handler{create: create, update: update, archive: archive, addVer: addVer, publish: publish, list: list, get: get}
}

type createAgentRequest struct {
	Name         string `json:"name" binding:"required"`
	SystemPrompt string `json:"system_prompt" binding:"required"`
	ModelName    string `json:"model_name" binding:"required"`
}

type addVersionRequest struct {
	SystemPrompt     string            `json:"system_prompt" binding:"required"`
	ModelName        string            `json:"model_name" binding:"required"`
	ModelConfig      model.ModelConfig `json:"model_config"`
	ToolIDs          []string          `json:"tool_ids"`
	KnowledgeBaseIDs []string          `json:"knowledge_base_ids"`
}

type publishRequest struct {
	VersionID string `json:"version_id" binding:"required"`
}

type updateAgentRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *Handler) CreateAgent(c *gin.Context) {
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
	var req createAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	agent, err := h.create.Handle(c.Request.Context(), wsID, projectID, req.Name, req.SystemPrompt, req.ModelName)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": agent.ID.String(), "name": agent.Name.String(), "status": agent.Status})
}

func (h *Handler) ListAgents(c *gin.Context) {
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

func (h *Handler) GetAgent(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	agentID, err := kernel.ParseAgentID(c.Param("agentId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid agent id", http.StatusBadRequest)
		return
	}
	item, err := h.get.Handle(c.Request.Context(), wsID, agentID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, item)
}

func (h *Handler) UpdateAgent(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	agentID, err := kernel.ParseAgentID(c.Param("agentId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid agent id", http.StatusBadRequest)
		return
	}
	var req updateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	agent, err := h.update.Handle(c.Request.Context(), wsID, agentID, req.Name)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": agent.ID.String(), "name": agent.Name.String(), "status": agent.Status})
}

func (h *Handler) ArchiveAgent(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	agentID, err := kernel.ParseAgentID(c.Param("agentId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid agent id", http.StatusBadRequest)
		return
	}
	if err := h.archive.Handle(c.Request.Context(), wsID, agentID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"deleted": true})
}

func (h *Handler) AddVersion(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	agentID, err := kernel.ParseAgentID(c.Param("agentId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid agent id", http.StatusBadRequest)
		return
	}
	var req addVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	cfg := req.ModelConfig
	if cfg.MaxTokens == 0 && cfg.Temperature == 0 {
		cfg = model.DefaultModelConfig()
	}
	ver, err := h.addVer.Handle(c.Request.Context(), wsID, agentID, req.SystemPrompt, req.ModelName, cfg, req.ToolIDs, req.KnowledgeBaseIDs)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"id": ver.ID.String(), "version": ver.Version})
}

func (h *Handler) Publish(c *gin.Context) {
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	agentID, err := kernel.ParseAgentID(c.Param("agentId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid agent id", http.StatusBadRequest)
		return
	}
	var req publishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	versionID, err := kernel.ParseAgentVersionID(req.VersionID)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid version id", http.StatusBadRequest)
		return
	}
	if err := h.publish.Handle(c.Request.Context(), wsID, agentID, versionID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"published": true})
}

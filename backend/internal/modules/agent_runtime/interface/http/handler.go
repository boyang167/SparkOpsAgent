package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	startChat            *command.StartChatHandler
	getConversation      *query.GetConversationHandler
	listConversations    *query.ListAgentConversationsHandler
	streamRun            *query.StreamRunHandler
}

func NewHandler(
	startChat *command.StartChatHandler,
	getConversation *query.GetConversationHandler,
	listConversations *query.ListAgentConversationsHandler,
	streamRun *query.StreamRunHandler,
) *Handler {
	return &Handler{
		startChat: startChat, getConversation: getConversation,
		listConversations: listConversations, streamRun: streamRun,
	}
}

type chatRequest struct {
	Message        string `json:"message" binding:"required"`
	ConversationID string `json:"conversation_id"`
}

func (h *Handler) StartChat(c *gin.Context) {
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
	agentID, err := kernel.ParseAgentID(c.Param("agentId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid agent id", http.StatusBadRequest)
		return
	}
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	var conversationID *kernel.ConversationID
	if req.ConversationID != "" {
		id, err := kernel.ParseConversationID(req.ConversationID)
		if err != nil {
			httpx.Fail(c, "INVALID_REQUEST", "invalid conversation id", http.StatusBadRequest)
			return
		}
		conversationID = &id
	}
	result, err := h.startChat.Handle(c.Request.Context(), wsID, memberID, agentID, req.Message, conversationID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"run_id": result.RunID.String(), "conversation_id": result.ConversationID.String()})
}

func (h *Handler) GetConversation(c *gin.Context) {
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
	conversationID, err := kernel.ParseConversationID(c.Param("id"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid conversation id", http.StatusBadRequest)
		return
	}
	items, err := h.getConversation.Handle(c.Request.Context(), wsID, memberID, conversationID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) ListAgentConversations(c *gin.Context) {
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
	agentID, err := kernel.ParseAgentID(c.Param("agentId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid agent id", http.StatusBadRequest)
		return
	}
	items, err := h.listConversations.Handle(c.Request.Context(), wsID, memberID, agentID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) StreamRun(c *gin.Context) {
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
	runID, err := kernel.ParseRunID(c.Param("runId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid run id", http.StatusBadRequest)
		return
	}
	ch, initial, done, err := h.streamRun.Subscribe(c.Request.Context(), wsID, memberID, runID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	StreamSSE(c, ch, initial, done)
}

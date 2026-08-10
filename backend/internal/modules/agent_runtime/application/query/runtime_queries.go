package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/stream"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MessageView struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type ConversationSummaryView struct {
	ID        string `json:"id"`
	Preview   string `json:"preview"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ListAgentConversationsHandler struct {
	conversations repository.ConversationRepository
	messages      repository.MessageRepository
}

func NewListAgentConversationsHandler(
	conversations repository.ConversationRepository,
	messages repository.MessageRepository,
) *ListAgentConversationsHandler {
	return &ListAgentConversationsHandler{conversations: conversations, messages: messages}
}

func (h *ListAgentConversationsHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	agentID kernel.AgentID,
) ([]ConversationSummaryView, error) {
	items, err := h.conversations.ListByAgent(ctx, agentID, memberID, wsID)
	if err != nil {
		return nil, err
	}
	out := make([]ConversationSummaryView, 0, len(items))
	for _, item := range items {
		preview, err := h.messages.FirstUserMessage(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, ConversationSummaryView{
			ID:        item.ID.String(),
			Preview:   truncatePreview(preview, 80),
			CreatedAt: item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: item.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return out, nil
}

func truncatePreview(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max]) + "..."
}

type GetConversationHandler struct {
	conversations repository.ConversationRepository
	messages      repository.MessageRepository
}

func NewGetConversationHandler(conversations repository.ConversationRepository, messages repository.MessageRepository) *GetConversationHandler {
	return &GetConversationHandler{conversations: conversations, messages: messages}
}

func (h *GetConversationHandler) Handle(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, conversationID kernel.ConversationID) ([]MessageView, error) {
	conversation, err := h.conversations.FindByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if conversation.WorkspaceID != wsID || conversation.MemberID != memberID {
		return nil, domain.ErrAccessDenied
	}
	items, err := h.messages.ListByConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	out := make([]MessageView, 0, len(items))
	for _, msg := range items {
		out = append(out, MessageView{
			ID: msg.ID.String(), Role: string(msg.Role), Content: msg.Content,
			CreatedAt: msg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return out, nil
}

type StreamRunHandler struct {
	conversations repository.ConversationRepository
	runs          repository.RunRepository
	hub           *stream.Hub
}

func NewStreamRunHandler(conversations repository.ConversationRepository, runs repository.RunRepository, hub *stream.Hub) *StreamRunHandler {
	return &StreamRunHandler{conversations: conversations, runs: runs, hub: hub}
}

func (h *StreamRunHandler) Subscribe(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID, runID kernel.RunID) (<-chan string, string, bool, error) {
	run, err := h.runs.FindByID(ctx, runID)
	if err != nil {
		return nil, "", false, err
	}
	conversation, err := h.conversations.FindByID(ctx, run.ConversationID)
	if err != nil {
		return nil, "", false, err
	}
	if conversation.WorkspaceID != wsID || conversation.MemberID != memberID {
		return nil, "", false, domain.ErrAccessDenied
	}
	ch, output, done := h.hub.Subscribe(runID)
	if done && output == "" {
		output = run.Output
	}
	return ch, output, done, nil
}

package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/stream"
	integrationport "github.com/sparkops/sparkops-agent/backend/internal/modules/integration/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"go.uber.org/zap"
)

type StartChatResult struct {
	RunID          kernel.RunID
	ConversationID kernel.ConversationID
}

type StartChatHandler struct {
	conversations repository.ConversationRepository
	messages      repository.MessageRepository
	runs          repository.RunRepository
	agents        port.AgentReader
	knowledge     port.KnowledgeRetriever
	llm           integrationport.LLMClient
	hub           *stream.Hub
	bus           eventbus.Bus
	tx            postgres.Transactor
	logger        *zap.Logger
}

func NewStartChatHandler(
	conversations repository.ConversationRepository,
	messages repository.MessageRepository,
	runs repository.RunRepository,
	agents port.AgentReader,
	knowledge port.KnowledgeRetriever,
	llm integrationport.LLMClient,
	hub *stream.Hub,
	bus eventbus.Bus,
	tx postgres.Transactor,
	logger *zap.Logger,
) *StartChatHandler {
	return &StartChatHandler{
		conversations: conversations, messages: messages, runs: runs,
		agents: agents, knowledge: knowledge, llm: llm, hub: hub, bus: bus, tx: tx, logger: logger,
	}
}

func (h *StartChatHandler) Handle(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	memberID kernel.MemberID,
	agentID kernel.AgentID,
	message string,
	conversationID *kernel.ConversationID,
) (*StartChatResult, error) {
	inWs, err := h.agents.AgentInWorkspace(ctx, agentID, wsID)
	if err != nil {
		return nil, err
	}
	if !inWs {
		return nil, domain.ErrAccessDenied
	}

	version, err := h.agents.GetPublishedVersion(ctx, agentID)
	if err != nil {
		return nil, err
	}

	var conversation *model.Conversation
	if conversationID != nil {
		conversation, err = h.conversations.FindByID(ctx, *conversationID)
		if err != nil {
			return nil, err
		}
		if conversation.AgentID != agentID || conversation.MemberID != memberID || conversation.WorkspaceID != wsID {
			return nil, domain.ErrAccessDenied
		}
	} else {
		conversation = model.NewConversation(agentID, memberID, wsID)
		if err := h.conversations.Save(ctx, conversation); err != nil {
			return nil, err
		}
	}

	userMsg := model.NewMessage(conversation.ID, model.MessageRoleUser, message)
	if err := h.messages.Save(ctx, userMsg); err != nil {
		return nil, err
	}

	history, err := h.messages.ListByConversation(ctx, conversation.ID)
	if err != nil {
		return nil, err
	}

	run := model.NewRun(conversation.ID, version.VersionID, message)
	if err := h.runs.Save(ctx, run); err != nil {
		return nil, err
	}

	go h.executeRun(context.Background(), wsID, run, conversation, version, history, message)

	return &StartChatResult{RunID: run.ID, ConversationID: conversation.ID}, nil
}

func (h *StartChatHandler) executeRun(
	ctx context.Context,
	wsID kernel.WorkspaceID,
	run *model.Run,
	conversation *model.Conversation,
	version *port.PublishedAgentVersion,
	history []*model.Message,
	userQuery string,
) {
	llmMessages := make([]integrationport.Message, 0, len(history)+1)
	systemPrompt := version.SystemPrompt
	if h.knowledge != nil && len(version.KnowledgeBaseIDs) > 0 {
		snippets, err := h.knowledge.Retrieve(ctx, wsID, version.KnowledgeBaseIDs, userQuery, 5)
		if err != nil {
			h.logger.Warn("knowledge retrieve failed", zap.Error(err))
		} else if rag := acl.BuildRAGContext(snippets); rag != "" {
			if systemPrompt != "" {
				systemPrompt = systemPrompt + "\n\n" + rag
			} else {
				systemPrompt = rag
			}
		}
	}
	if systemPrompt != "" {
		llmMessages = append(llmMessages, integrationport.Message{Role: "system", Content: systemPrompt})
	}
	for _, msg := range history {
		llmMessages = append(llmMessages, integrationport.Message{Role: string(msg.Role), Content: msg.Content})
	}

	req := integrationport.ChatRequest{
		Model: version.ModelName, Messages: llmMessages,
		Temperature: version.Temperature, MaxTokens: version.MaxTokens,
	}
	err := h.llm.ChatStream(ctx, req, func(delta string) error {
		run.AppendOutput(delta)
		h.hub.Publish(run.ID, delta)
		return nil
	})
	if err != nil {
		h.logger.Error("chat stream failed", zap.Error(err), zap.String("run_id", run.ID.String()))
		run.Fail()
		_ = h.runs.Save(ctx, run)
		h.hub.Complete(run.ID, run.Output)
		return
	}

	run.Complete(run.Output)
	assistant := model.NewMessage(conversation.ID, model.MessageRoleAssistant, run.Output)
	if err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.runs.Save(txCtx, run); err != nil {
			return err
		}
		if err := h.messages.Save(txCtx, assistant); err != nil {
			return err
		}
		return h.bus.Publish(txCtx, event.NewRunCompleted(
			run.ID,
			conversation.ID,
			conversation.AgentID,
			version.ProjectID,
			conversation.MemberID,
			conversation.WorkspaceID,
		))
	}); err != nil {
		h.logger.Error("persist completed run failed", zap.Error(err))
		return
	}
	h.hub.Complete(run.ID, run.Output)
}

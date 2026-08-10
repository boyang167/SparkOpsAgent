package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ConversationRepository interface {
	Save(ctx context.Context, conversation *model.Conversation) error
	FindByID(ctx context.Context, id kernel.ConversationID) (*model.Conversation, error)
	ListByAgent(ctx context.Context, agentID kernel.AgentID, memberID kernel.MemberID, wsID kernel.WorkspaceID) ([]*model.Conversation, error)
}

type MessageRepository interface {
	Save(ctx context.Context, message *model.Message) error
	ListByConversation(ctx context.Context, conversationID kernel.ConversationID) ([]*model.Message, error)
	FirstUserMessage(ctx context.Context, conversationID kernel.ConversationID) (string, error)
}

type RunRepository interface {
	Save(ctx context.Context, run *model.Run) error
	FindByID(ctx context.Context, id kernel.RunID) (*model.Run, error)
}

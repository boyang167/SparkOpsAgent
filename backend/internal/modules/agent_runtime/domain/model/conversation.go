package model

import (
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
)

type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
)

type Conversation struct {
	ID          kernel.ConversationID
	AgentID     kernel.AgentID
	MemberID    kernel.MemberID
	WorkspaceID kernel.WorkspaceID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewConversation(agentID kernel.AgentID, memberID kernel.MemberID, wsID kernel.WorkspaceID) *Conversation {
	now := time.Now().UTC()
	return &Conversation{
		ID:          kernel.NewConversationID(),
		AgentID:     agentID,
		MemberID:    memberID,
		WorkspaceID: wsID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

type Message struct {
	ID             kernel.MessageID
	ConversationID kernel.ConversationID
	Role           MessageRole
	Content        string
	CreatedAt      time.Time
}

func NewMessage(conversationID kernel.ConversationID, role MessageRole, content string) *Message {
	return &Message{
		ID:             kernel.NewMessageID(),
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		CreatedAt:      time.Now().UTC(),
	}
}

type Run struct {
	ID             kernel.RunID
	ConversationID kernel.ConversationID
	AgentVersionID kernel.AgentVersionID
	Status         RunStatus
	Input          string
	Output         string
	PromptTokens   int
	CompletionTokens int
	CreatedAt      time.Time
	CompletedAt    *time.Time
}

func NewRun(conversationID kernel.ConversationID, versionID kernel.AgentVersionID, input string) *Run {
	return &Run{
		ID:             kernel.NewRunID(),
		ConversationID: conversationID,
		AgentVersionID: versionID,
		Status:         RunStatusRunning,
		Input:          input,
		CreatedAt:      time.Now().UTC(),
	}
}

func (r *Run) Complete(output string) {
	now := time.Now().UTC()
	r.Status = RunStatusCompleted
	r.Output = output
	r.CompletedAt = &now
}

func (r *Run) Fail() {
	now := time.Now().UTC()
	r.Status = RunStatusFailed
	r.CompletedAt = &now
}

func (r *Run) IsRunning() bool { return r.Status == RunStatusRunning }

func (r *Run) AppendOutput(delta string) { r.Output += delta }

func (r *Run) EnsureStreamable() error {
	if r.Status != RunStatusRunning && r.Status != RunStatusCompleted {
		return domain.ErrRunNotRunning
	}
	return nil
}

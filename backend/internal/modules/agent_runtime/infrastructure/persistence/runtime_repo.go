package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func AutoMigrate(db *gorm.DB) error {
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS agent_runtime").Error; err != nil {
			return err
		}
	}
	return db.AutoMigrate(&conversationModel{}, &messageModel{}, &runModel{})
}

type conversationModel struct {
	ID          string `gorm:"column:id;primaryKey"`
	AgentID     string
	MemberID    string
	WorkspaceID string
	postgres.AuditColumns
}

func (conversationModel) TableName() string { return "agent_runtime.conversations" }

type messageModel struct {
	ID             string `gorm:"column:id;primaryKey"`
	ConversationID string
	Role           string
	Content        string
	postgres.AuditColumns
}

func (messageModel) TableName() string { return "agent_runtime.messages" }

type runModel struct {
	ID               string `gorm:"column:id;primaryKey"`
	ConversationID   string
	AgentVersionID   string
	Status           string
	Input            string
	Output           string
	PromptTokens     int
	CompletionTokens int
	CompletedAt      *time.Time
	postgres.AuditColumns
}

func (runModel) TableName() string { return "agent_runtime.runs" }

type Store struct{ db *gorm.DB }

func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

func (s *Store) SaveConversation(ctx context.Context, c *model.Conversation) error {
	row := conversationModel{
		ID: c.ID.String(), AgentID: c.AgentID.String(), MemberID: c.MemberID.String(),
		WorkspaceID: c.WorkspaceID.String(),
		AuditColumns: postgres.AuditColumns{
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		},
	}
	if row.UpdatedAt.IsZero() {
		row.UpdatedAt = row.CreatedAt
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

func (s *Store) ListConversationsByAgent(
	ctx context.Context,
	agentID kernel.AgentID,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
) ([]*model.Conversation, error) {
	var rows []conversationModel
	err := s.db.WithContext(ctx).
		Where("agent_id = ? AND member_id = ? AND workspace_id = ?", agentID.String(), memberID.String(), wsID.String()).
		Order("updated_at desc").
		Limit(50).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Conversation, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowToConversation(row))
	}
	return out, nil
}

func (s *Store) FindFirstUserMessage(ctx context.Context, conversationID kernel.ConversationID) (string, error) {
	var row messageModel
	err := s.db.WithContext(ctx).
		Where("conversation_id = ? AND role = ?", conversationID.String(), string(model.MessageRoleUser)).
		Order("created_at asc").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return row.Content, nil
}

func (s *Store) FindConversationByID(ctx context.Context, id kernel.ConversationID) (*model.Conversation, error) {
	var row conversationModel
	err := s.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrConversationNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToConversation(row), nil
}

func (s *Store) SaveMessage(ctx context.Context, m *model.Message) error {
	row := messageModel{
		ID: m.ID.String(), ConversationID: m.ConversationID.String(),
		Role: string(m.Role), Content: m.Content,
		AuditColumns: postgres.AuditColumns{CreatedAt: m.CreatedAt, UpdatedAt: m.CreatedAt},
	}
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&conversationModel{}).
		Where("id = ?", m.ConversationID.String()).
		Update("updated_at", now).Error
}

func (s *Store) ListMessagesByConversation(ctx context.Context, conversationID kernel.ConversationID) ([]*model.Message, error) {
	var rows []messageModel
	err := s.db.WithContext(ctx).Where("conversation_id = ?", conversationID.String()).
		Order("created_at asc").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Message, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowToMessage(row))
	}
	return out, nil
}

func (s *Store) SaveRun(ctx context.Context, run *model.Run) error {
	row := runToModel(run)
	return s.db.WithContext(ctx).Save(&row).Error
}

func (s *Store) FindRunByID(ctx context.Context, id kernel.RunID) (*model.Run, error) {
	var row runModel
	err := s.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToRun(row), nil
}

func rowToConversation(row conversationModel) *model.Conversation {
	cid, _ := kernel.ParseConversationID(row.ID)
	agentID, _ := kernel.ParseAgentID(row.AgentID)
	memberID, _ := kernel.ParseMemberID(row.MemberID)
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	return &model.Conversation{
		ID: cid, AgentID: agentID, MemberID: memberID, WorkspaceID: wsID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func rowToMessage(row messageModel) *model.Message {
	msgID, _ := kernel.ParseMessageID(row.ID)
	cid, _ := kernel.ParseConversationID(row.ConversationID)
	return &model.Message{
		ID: msgID, ConversationID: cid, Role: model.MessageRole(row.Role),
		Content: row.Content, CreatedAt: row.CreatedAt,
	}
}

func runToModel(run *model.Run) runModel {
	return runModel{
		ID: run.ID.String(), ConversationID: run.ConversationID.String(),
		AgentVersionID: run.AgentVersionID.String(), Status: string(run.Status),
		Input: run.Input, Output: run.Output, PromptTokens: run.PromptTokens,
		CompletionTokens: run.CompletionTokens, CompletedAt: run.CompletedAt,
		AuditColumns: postgres.AuditColumns{CreatedAt: run.CreatedAt, UpdatedAt: run.CreatedAt},
	}
}

func rowToRun(row runModel) *model.Run {
	runID, _ := kernel.ParseRunID(row.ID)
	cid, _ := kernel.ParseConversationID(row.ConversationID)
	verID, _ := kernel.ParseAgentVersionID(row.AgentVersionID)
	return &model.Run{
		ID: runID, ConversationID: cid, AgentVersionID: verID, Status: model.RunStatus(row.Status),
		Input: row.Input, Output: row.Output, PromptTokens: row.PromptTokens,
		CompletionTokens: row.CompletionTokens, CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt,
	}
}

type conversationRepo struct{ store *Store }
type messageRepo struct{ store *Store }
type runRepo struct{ store *Store }

func NewConversationRepository(store *Store) repository.ConversationRepository {
	return &conversationRepo{store: store}
}
func NewMessageRepository(store *Store) repository.MessageRepository { return &messageRepo{store: store} }
func NewRunRepository(store *Store) repository.RunRepository         { return &runRepo{store: store} }

func (r *conversationRepo) Save(ctx context.Context, c *model.Conversation) error {
	return r.store.SaveConversation(ctx, c)
}
func (r *conversationRepo) FindByID(ctx context.Context, id kernel.ConversationID) (*model.Conversation, error) {
	return r.store.FindConversationByID(ctx, id)
}
func (r *conversationRepo) ListByAgent(
	ctx context.Context,
	agentID kernel.AgentID,
	memberID kernel.MemberID,
	wsID kernel.WorkspaceID,
) ([]*model.Conversation, error) {
	return r.store.ListConversationsByAgent(ctx, agentID, memberID, wsID)
}
func (r *messageRepo) Save(ctx context.Context, m *model.Message) error { return r.store.SaveMessage(ctx, m) }
func (r *messageRepo) ListByConversation(ctx context.Context, id kernel.ConversationID) ([]*model.Message, error) {
	return r.store.ListMessagesByConversation(ctx, id)
}
func (r *messageRepo) FirstUserMessage(ctx context.Context, id kernel.ConversationID) (string, error) {
	return r.store.FindFirstUserMessage(ctx, id)
}
func (r *runRepo) Save(ctx context.Context, run *model.Run) error { return r.store.SaveRun(ctx, run) }
func (r *runRepo) FindByID(ctx context.Context, id kernel.RunID) (*model.Run, error) {
	return r.store.FindRunByID(ctx, id)
}

var (
	_ repository.ConversationRepository = (*conversationRepo)(nil)
	_ repository.MessageRepository      = (*messageRepo)(nil)
	_ repository.RunRepository          = (*runRepo)(nil)
)

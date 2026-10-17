package command

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/model"
	runtimeport "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/port"
	runtimerepository "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/repository"
	runtimepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/infrastructure/stream"
	integrationport "github.com/sparkops/sparkops-agent/backend/internal/modules/integration/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type atomicChatLLM struct{}

func (atomicChatLLM) ChatStream(_ context.Context, _ integrationport.ChatRequest, onDelta func(string) error) error {
	return onDelta("answer")
}
func (atomicChatLLM) ListModels(context.Context) ([]integrationport.ModelInfo, error) {
	return nil, nil
}

func TestCompletedRunAssistantMessageAndOutboxAreAtomic(t *testing.T) {
	db := openRuntimeAtomicityDB(t)
	store := runtimepersistence.NewStore(db)
	conversations := runtimepersistence.NewConversationRepository(store)
	messages := runtimepersistence.NewMessageRepository(store)
	runs := runtimepersistence.NewRunRepository(store)
	conversation := model.NewConversation(kernel.NewAgentID(), kernel.NewMemberID(), kernel.NewWorkspaceID())
	require.NoError(t, conversations.Save(context.Background(), conversation))
	version := &runtimeport.PublishedAgentVersion{
		VersionID: kernel.NewAgentVersionID(),
		ProjectID: kernel.NewProjectID(),
		ModelName: "test",
	}

	run := model.NewRun(conversation.ID, version.VersionID, "question")
	require.NoError(t, runs.Save(context.Background(), run))
	handler := atomicStartChatHandler(
		db, conversations, messages, runs, sharedevents.NewRegistry(),
	)
	handler.executeRun(context.Background(), conversation.WorkspaceID, run, conversation, version, nil, "question")
	stored, err := runs.FindByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, model.RunStatusRunning, stored.Status)
	requireRuntimeAtomicityCount(t, db, "agent_runtime.messages", 0)
	requireRuntimeAtomicityCount(t, db, "platform.event_outbox", 0)

	registry := sharedevents.NewRegistry()
	require.NoError(t, event.Register(registry))
	run = model.NewRun(conversation.ID, version.VersionID, "question")
	require.NoError(t, runs.Save(context.Background(), run))
	handler = atomicStartChatHandler(db, conversations, messages, runs, registry)
	handler.executeRun(context.Background(), conversation.WorkspaceID, run, conversation, version, nil, "question")
	stored, err = runs.FindByID(context.Background(), run.ID)
	require.NoError(t, err)
	require.Equal(t, model.RunStatusCompleted, stored.Status)
	requireRuntimeAtomicityCount(t, db, "agent_runtime.messages", 1)
	requireRuntimeAtomicityCount(t, db, "platform.event_outbox", 1)
}

func atomicStartChatHandler(
	db *gorm.DB,
	conversations runtimerepository.ConversationRepository,
	messages runtimerepository.MessageRepository,
	runs runtimerepository.RunRepository,
	registry *sharedevents.Registry,
) *StartChatHandler {
	return &StartChatHandler{
		conversations: conversations,
		messages:      messages,
		runs:          runs,
		llm:           atomicChatLLM{},
		hub:           stream.NewHub(),
		bus: messaging.NewReliableBus(
			registry,
			messagingpersistence.NewOutboxStore(db),
			nil, nil, nil,
		),
		tx:     postgres.NewTransactor(db),
		logger: zap.NewNop(),
	}
}

func openRuntimeAtomicityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS agent_runtime").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE agent_runtime.conversations (
			id TEXT PRIMARY KEY, agent_id TEXT, member_id TEXT, workspace_id TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
		CREATE TABLE agent_runtime.messages (
			id TEXT PRIMARY KEY, conversation_id TEXT, role TEXT, content TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
		CREATE TABLE agent_runtime.runs (
			id TEXT PRIMARY KEY, conversation_id TEXT, agent_version_id TEXT, status TEXT,
			input TEXT, output TEXT, prompt_tokens INTEGER, completion_tokens INTEGER,
			completed_at DATETIME, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		)
	`).Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	return db
}

func requireRuntimeAtomicityCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Count(&count).Error)
	require.Equal(t, want, count)
}

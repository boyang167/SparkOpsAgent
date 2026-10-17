package command_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/application/command"
	agentevent "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/agent/domain/model"
	agentpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/agent/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type atomicAgentProjectChecker struct{}

func (atomicAgentProjectChecker) ProjectInWorkspace(context.Context, kernel.ProjectID, kernel.WorkspaceID) (bool, error) {
	return true, nil
}
func (atomicAgentProjectChecker) ProjectIsActive(context.Context, kernel.ProjectID) (bool, error) {
	return true, nil
}

func TestAgentPublishAndOutboxAreAtomic(t *testing.T) {
	db := openAgentAtomicityDB(t)
	repo := agentpersistence.NewAgentRepository(db)
	name, err := model.NewAgentName("release-agent")
	require.NoError(t, err)
	agent := model.NewAgent(kernel.NewProjectID(), name)
	version, err := agent.AddVersion("prompt", "model", model.DefaultModelConfig(), nil, nil)
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), agent))

	handler := command.NewPublishAgentHandler(
		repo,
		atomicAgentProjectChecker{},
		messaging.NewReliableBus(
			sharedevents.NewRegistry(),
			messagingpersistence.NewOutboxStore(db),
			nil, nil, nil,
		),
		postgres.NewTransactor(db),
	)
	err = handler.Handle(context.Background(), kernel.NewWorkspaceID(), agent.ID, version.ID)
	require.Error(t, err)
	stored, findErr := repo.FindByID(context.Background(), agent.ID)
	require.NoError(t, findErr)
	require.Equal(t, model.AgentStatusDraft, stored.Status)
	requireAgentAtomicityCount(t, db, "platform.event_outbox", 0)

	registry := sharedevents.NewRegistry()
	require.NoError(t, agentevent.Register(registry))
	handler = command.NewPublishAgentHandler(
		repo,
		atomicAgentProjectChecker{},
		messaging.NewReliableBus(
			registry,
			messagingpersistence.NewOutboxStore(db),
			nil, nil, nil,
		),
		postgres.NewTransactor(db),
	)
	require.NoError(t, handler.Handle(context.Background(), kernel.NewWorkspaceID(), agent.ID, version.ID))
	stored, err = repo.FindByID(context.Background(), agent.ID)
	require.NoError(t, err)
	require.Equal(t, model.AgentStatusPublished, stored.Status)
	requireAgentAtomicityCount(t, db, "platform.event_outbox", 1)
}

func openAgentAtomicityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS agent").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE agent.agents (
			id TEXT PRIMARY KEY, project_id TEXT, name TEXT, status TEXT, current_version_id TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
		CREATE TABLE agent.agent_versions (
			id TEXT PRIMARY KEY, agent_id TEXT, version INTEGER, system_prompt TEXT, model_name TEXT,
			model_config BLOB, tool_ids BLOB, knowledge_base_ids BLOB,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		)
	`).Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	return db
}

func requireAgentAtomicityCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Count(&count).Error)
	require.Equal(t, want, count)
}

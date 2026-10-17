package command_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	identityevent "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/event"
	identitypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

func TestRegisterMemberRollsBackBusinessWriteWhenOutboxWriteFails(t *testing.T) {
	db := openIdentityAtomicityDB(t)
	registry := sharedevents.NewRegistry() // Missing codec makes Publish fail after member Save.
	bus := messaging.NewReliableBus(registry, messagingpersistence.NewOutboxStore(db), nil, nil, nil)
	handler := command.NewRegisterMemberHandler(
		identitypersistence.NewMemberRepository(db),
		bus,
		postgres.NewTransactor(db),
	)

	_, err := handler.Handle(context.Background(), command.RegisterMemberInput{
		Email: "rollback@example.com", Password: "password", DisplayName: "Rollback",
	})
	require.Error(t, err)
	requireTableCount(t, db, "identity.members", 0)
	requireTableCount(t, db, "platform.event_outbox", 0)
}

func TestRegisterMemberCommitsBusinessWriteAndOutboxTogether(t *testing.T) {
	db := openIdentityAtomicityDB(t)
	registry := sharedevents.NewRegistry()
	require.NoError(t, identityevent.Register(registry))
	bus := messaging.NewReliableBus(registry, messagingpersistence.NewOutboxStore(db), nil, nil, nil)
	handler := command.NewRegisterMemberHandler(
		identitypersistence.NewMemberRepository(db),
		bus,
		postgres.NewTransactor(db),
	)

	_, err := handler.Handle(context.Background(), command.RegisterMemberInput{
		Email: "commit@example.com", Password: "password", DisplayName: "Commit",
	})
	require.NoError(t, err)
	requireTableCount(t, db, "identity.members", 1)
	requireTableCount(t, db, "platform.event_outbox", 1)

	var eventType string
	require.NoError(t, db.Table("platform.event_outbox").Select("event_type").Scan(&eventType).Error)
	require.Equal(t, identityevent.MemberRegisteredName, eventType)
}

func openIdentityAtomicityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS identity").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE identity.members (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)
	`).Error)
	require.NoError(t, messagingpersistence.AutoMigrate(db))
	return db
}

func requireTableCount(t *testing.T, db *gorm.DB, table string, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Count(&count).Error)
	require.Equal(t, want, count)
}

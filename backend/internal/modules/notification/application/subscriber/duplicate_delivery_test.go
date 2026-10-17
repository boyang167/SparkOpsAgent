package subscriber_test

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	runtimeevent "github.com/sparkops/sparkops-agent/backend/internal/modules/agent_runtime/domain/event"
	deliveryevent "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/application/subscriber"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/infrastructure/acl"
	notificationpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/notification/infrastructure/persistence"
	workflowevent "github.com/sparkops/sparkops-agent/backend/internal/modules/workflow/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	messagingpersistence "github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type duplicateTestTransport struct{}

func (duplicateTestTransport) Declare(context.Context, []messaging.Subscription) error { return nil }
func (duplicateTestTransport) Publish(context.Context, events.Envelope) error          { return nil }
func (duplicateTestTransport) Consume(context.Context, messaging.Subscription, messaging.DeliveryHandler) error {
	return nil
}
func (duplicateTestTransport) PublishRetry(context.Context, messaging.Subscription, events.Envelope, messaging.RetryDelay, int) error {
	return errors.New("unexpected retry")
}
func (duplicateTestTransport) PublishDeadLetter(context.Context, messaging.Subscription, events.Envelope, string) error {
	return nil
}
func (duplicateTestTransport) Close() error { return nil }

func TestNotificationSubscribersSkipDuplicateDeliveries(t *testing.T) {
	db := openNotificationInboxDB(t)
	registry := events.NewRegistry()
	require.NoError(t, runtimeevent.Register(registry))
	require.NoError(t, deliveryevent.Register(registry))
	require.NoError(t, workflowevent.Register(registry))

	repo := notificationpersistence.NewNotificationRepository(db)
	create := command.NewCreateNotificationHandler(repo)
	consumer := messaging.NewInboxConsumer(
		postgres.NewTransactor(db),
		messagingpersistence.NewInboxStore(db),
		registry,
		duplicateTestTransport{},
		nil,
		nil,
	)
	memberID := kernel.NewMemberID()
	workspaceID := kernel.NewWorkspaceID()
	projectID := kernel.NewProjectID()
	cases := []struct {
		name       string
		subscriber string
		event      events.Event
		handler    messaging.Handler
	}{
		{
			name:       "run completed",
			subscriber: "notification.run-completed",
			event: runtimeevent.NewRunCompleted(
				kernel.NewRunID(), kernel.NewConversationID(), kernel.NewAgentID(),
				projectID, memberID, workspaceID,
			),
			handler: subscriber.NewOnRunCompleted(create, acl.NewAgentACL(db)).Handle,
		},
		{
			name:       "workflow run completed",
			subscriber: "notification.workflow-run-completed",
			event: workflowevent.NewWorkflowRunCompleted(
				kernel.NewWorkflowRunID(), kernel.NewWorkflowID(), projectID,
				memberID, workspaceID, "release", "success",
			),
			handler: subscriber.NewOnWorkflowRunCompleted(create).Handle,
		},
		{
			name:       "build completed",
			subscriber: "notification.build-completed",
			event: deliveryevent.NewBuildCompleted(
				kernel.NewBuildID(), projectID, memberID, workspaceID, "api", "success", 1,
			),
			handler: subscriber.NewOnBuildCompleted(create).Handle,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			envelope, err := registry.Encode(context.Background(), tt.event)
			require.NoError(t, err)
			subscription := messaging.Subscription{
				EventName:  tt.event.EventName(),
				Version:    1,
				Subscriber: tt.subscriber,
				Handler:    tt.handler,
			}
			require.NoError(t, consumer.Handle(context.Background(), subscription, envelope, 0))
			require.NoError(t, consumer.Handle(context.Background(), subscription, envelope, 0))
		})
	}

	var count int64
	require.NoError(t, db.Table("notification.notifications").Count(&count).Error)
	require.Equal(t, int64(3), count)
}

func openNotificationInboxDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(2)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	for _, schema := range []string{"platform", "notification", "agent"} {
		require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS "+schema).Error)
	}
	require.NoError(t, db.Exec(`
		CREATE TABLE platform.event_inbox (
			event_id TEXT NOT NULL,
			subscriber TEXT NOT NULL,
			event_type TEXT NOT NULL,
			processed_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL,
			PRIMARY KEY(event_id, subscriber)
		);
		CREATE TABLE notification.notifications (
			id TEXT PRIMARY KEY, member_id TEXT, workspace_id TEXT, type TEXT,
			title TEXT, body TEXT, link TEXT, read_at DATETIME,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
		CREATE TABLE agent.agents (id TEXT PRIMARY KEY, name TEXT, project_id TEXT)
	`).Error)
	return db
}

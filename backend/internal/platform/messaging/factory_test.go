package messaging_test

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestFactoryInMemoryIsSynchronousWithoutRabbitMQ(t *testing.T) {
	cfg := &config.Config{
		Messaging: config.MessagingConfig{
			Driver: "inmemory",
			AMQP: config.AMQPConfig{
				URL: "amqp://this-host-must-not-be-dialed.invalid",
			},
		},
	}
	registry := events.NewRegistry()
	require.NoError(t, events.RegisterJSON[factoryEvent](
		registry,
		"test.FactoryEvent",
		1,
		func(factoryEvent) events.AggregateRef { return events.AggregateRef{} },
	))
	factory, err := messaging.NewFromConfig(
		cfg,
		nil,
		registry,
		postgres.NewDirectTransactor(),
		zap.NewNop(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, factory.Close()) })
	require.Nil(t, factory.Operations)

	handled := false
	require.NoError(t, factory.Bus.Subscribe(
		"test.FactoryEvent",
		"test.factory-handler",
		func(context.Context, events.Event) error {
			handled = true
			return nil
		},
	))
	require.NoError(t, factory.Start(context.Background()))
	require.NoError(t, factory.Bus.Publish(context.Background(), factoryEvent{
		BaseEvent: events.NewBaseEvent("test.FactoryEvent"),
	}))
	require.True(t, handled)
}

func TestFactoryInMemoryWithDatabaseDoesNotCreateOperationRuntime(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	cfg := &config.Config{
		Messaging: config.MessagingConfig{Driver: "inmemory"},
	}
	registry := events.NewRegistry()
	require.NoError(t, events.RegisterJSON[factoryEvent](
		registry,
		"test.FactoryEvent",
		1,
		func(factoryEvent) events.AggregateRef { return events.AggregateRef{} },
	))

	factory, err := messaging.NewFromConfig(
		cfg,
		db,
		registry,
		postgres.NewTransactor(db),
		zap.NewNop(),
	)
	require.NoError(t, err)
	require.Nil(t, factory.Operations)
	require.NoError(t, factory.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, factory.Close()) })

	handled := false
	require.NoError(t, factory.Bus.Subscribe(
		"test.FactoryEvent",
		"test.factory-db-handler",
		func(context.Context, events.Event) error {
			handled = true
			return nil
		},
	))
	require.NoError(t, factory.Bus.Publish(context.Background(), factoryEvent{
		BaseEvent: events.NewBaseEvent("test.FactoryEvent"),
	}))
	require.True(t, handled)
}

type factoryEvent struct {
	events.BaseEvent
}

func (factoryEvent) OccurredAt() time.Time {
	return time.Now().UTC()
}

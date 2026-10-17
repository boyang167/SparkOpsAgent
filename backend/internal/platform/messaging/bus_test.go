package messaging_test

import (
	"context"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
)

func TestInMemoryBus_Publish_delivers_to_subscriber(t *testing.T) {
	bus := messaging.NewInMemoryBus()
	var got string
	require.NoError(t, bus.Subscribe("test.event", "test.handler", func(_ context.Context, ev events.Event) error {
		got = ev.EventName()
		return nil
	}))

	err := bus.Publish(context.Background(), events.NewBaseEvent("test.event"))
	require.NoError(t, err)
	require.Equal(t, "test.event", got)
}

func TestInMemoryBusRejectsDuplicateNamedSubscription(t *testing.T) {
	bus := messaging.NewInMemoryBus(events.NewRegistry())
	handler := func(context.Context, events.Event) error { return nil }

	require.NoError(t, bus.Subscribe("test.event", "test.subscriber", handler))
	require.ErrorContains(t, bus.Subscribe("test.event", "test.subscriber", handler), "duplicate")
}

func TestInMemoryBusInjectsEventMetadata(t *testing.T) {
	bus := messaging.NewInMemoryBus()
	var got events.Metadata
	require.NoError(t, bus.Subscribe("test.event", "test.handler", func(ctx context.Context, _ events.Event) error {
		var ok bool
		got, ok = events.MetadataFromContext(ctx)
		require.True(t, ok)
		return nil
	}))

	require.NoError(t, bus.Publish(context.Background(), events.NewBaseEvent("test.event")))
	require.NotEmpty(t, got.EventID)
	require.Equal(t, "test.event", got.EventType)
	require.Equal(t, 1, got.EventVersion)
	require.Equal(t, "test.handler", got.Subscriber)
	require.Equal(t, got.EventID, got.CorrelationID)
}

func TestNoopQueue_Publish_subscribe(t *testing.T) {
	q := messaging.NewNoopQueue()
	ctx := context.Background()

	err := q.Subscribe(ctx, "build.sparkops", func(_ context.Context, payload []byte) error {
		require.Equal(t, []byte("hello"), payload)
		return nil
	})
	require.NoError(t, err)

	err = q.Publish(ctx, "build.sparkops", []byte("hello"))
	require.NoError(t, err)
}

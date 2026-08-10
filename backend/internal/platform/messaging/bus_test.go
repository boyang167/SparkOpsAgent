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
	bus.Subscribe("test.event", func(_ context.Context, ev events.Event) error {
		got = ev.EventName()
		return nil
	})

	err := bus.Publish(context.Background(), events.NewBaseEvent("test.event"))
	require.NoError(t, err)
	require.Equal(t, "test.event", got)
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

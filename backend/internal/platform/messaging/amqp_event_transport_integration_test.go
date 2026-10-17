//go:build integration

package messaging

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
)

func TestRabbitEventTransportRoutesOneEventToEachSubscriber(t *testing.T) {
	url := os.Getenv("AMQP_URL")
	if url == "" {
		t.Skip("AMQP_URL is required for RabbitMQ integration tests")
	}

	connection, err := amqp.Dial(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })

	suffix := fmt.Sprintf("event-transport-%d", time.Now().UnixNano())
	subscriptions := []Subscription{
		{
			EventName:  "delivery.BuildCompleted",
			Version:    1,
			Subscriber: "integration-" + suffix + "-one",
		},
		{
			EventName:  "delivery.BuildCompleted",
			Version:    1,
			Subscriber: "integration-" + suffix + "-two",
		},
	}
	for _, subscription := range subscriptions {
		topology := topologyFor(subscription)
		t.Cleanup(func() {
			channel, cleanupErr := connection.Channel()
			if cleanupErr != nil {
				return
			}
			defer channel.Close()
			_, _ = channel.QueueDelete(topology.Main.Name, false, false, false)
			for _, retry := range topology.Retry {
				_, _ = channel.QueueDelete(retry.Name, false, false, false)
			}
			_, _ = channel.QueueDelete(topology.DeadLetter.Name, false, false, false)
		})
	}

	metrics, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	transport, err := NewAMQPEventTransport(url, metrics)
	require.NoError(t, err)
	t.Cleanup(func() { _ = transport.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, transport.Declare(ctx, subscriptions))
	require.NoError(t, transport.Declare(ctx, subscriptions))
	require.NoError(t, transport.Publish(ctx, events.Envelope{
		ID:         uuid.NewString(),
		Type:       "delivery.BuildCompleted",
		Version:    1,
		OccurredAt: time.Now().UTC(),
		Payload:    []byte(`{"build_id":"build-123"}`),
	}))

	channel, err := connection.Channel()
	require.NoError(t, err)
	t.Cleanup(func() { _ = channel.Close() })
	for _, subscription := range subscriptions {
		delivery, ok, getErr := channel.Get(topologyFor(subscription).Main.Name, false)
		require.NoError(t, getErr)
		require.True(t, ok)
		require.Equal(t, suffix, delivery.MessageId)
		require.NoError(t, delivery.Ack(false))
	}
}

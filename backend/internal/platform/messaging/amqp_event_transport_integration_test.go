//go:build integration

package messaging

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
)

func TestRabbitEventTransportRoutesOneEventToEachSubscriber(t *testing.T) {
	url := requireIntegrationEnv(t, "AMQP_URL")

	connection, err := amqp.Dial(url)
	require.NoError(t, err)
	t.Logf("RabbitMQ AMQP server version: %v", connection.Properties["version"])
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
	envelope := events.Envelope{
		ID:         uuid.NewString(),
		Type:       "delivery.BuildCompleted",
		Version:    1,
		OccurredAt: time.Now().UTC(),
		Payload:    []byte(`{"build_id":"build-123"}`),
	}
	require.NoError(t, transport.Publish(ctx, envelope))

	channel, err := connection.Channel()
	require.NoError(t, err)
	t.Cleanup(func() { _ = channel.Close() })
	for _, subscription := range subscriptions {
		delivery, ok, getErr := channel.Get(topologyFor(subscription).Main.Name, false)
		require.NoError(t, getErr)
		require.True(t, ok)
		require.Equal(t, envelope.ID, delivery.MessageId)
		require.NoError(t, delivery.Ack(false))
	}
}

func TestRabbitEventTransportSameInstanceReconnectsPublisherAndConsumer(t *testing.T) {
	amqpURL := requireIntegrationEnv(t, "AMQP_URL")
	suffix := fmt.Sprintf("reconnect-%d", time.Now().UnixNano())
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

	transportInterface, err := NewAMQPEventTransport(amqpURL, nil)
	require.NoError(t, err)
	transport := transportInterface.(*amqpEventTransport)
	t.Cleanup(func() { _ = transport.Close() })
	for _, subscription := range subscriptions {
		cleanupRabbitSubscription(t, amqpURL, subscription)
	}

	received := []chan string{make(chan string, 1), make(chan string, 1)}
	require.NoError(t, transport.Declare(context.Background(), subscriptions))
	for index, subscription := range subscriptions {
		target := received[index]
		require.NoError(t, transport.Consume(
			context.Background(),
			subscription,
			func(_ context.Context, envelope events.Envelope, _ int) error {
				target <- envelope.ID
				return nil
			},
		))
	}

	transport.mu.Lock()
	failedConsumer := transport.consumerConnection
	failedPublisher := transport.publisherConnection
	transport.mu.Unlock()
	require.NoError(t, failedConsumer.CloseDeadline(time.Now().Add(time.Second)))
	require.NoError(t, failedPublisher.CloseDeadline(time.Now().Add(time.Second)))

	envelope := events.Envelope{
		ID:         uuid.NewString(),
		Type:       subscriptions[0].EventName,
		Version:    subscriptions[0].Version,
		OccurredAt: time.Now().UTC(),
		Payload:    []byte(`{"build_id":"reconnected"}`),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, transport.Publish(ctx, envelope))

	for index, target := range received {
		select {
		case eventID := <-target:
			require.Equal(t, envelope.ID, eventID)
		case <-ctx.Done():
			t.Fatalf("same transport did not restore consumer %d after connection loss", index)
		}
	}

	transport.mu.Lock()
	defer transport.mu.Unlock()
	require.NotSame(t, failedConsumer, transport.consumerConnection)
	require.NotSame(t, failedPublisher, transport.publisherConnection)
	require.Len(t, transport.registrations, len(subscriptions))
}

func cleanupRabbitSubscription(t *testing.T, amqpURL string, subscription Subscription) {
	t.Helper()
	t.Cleanup(func() {
		connection, err := amqp.Dial(amqpURL)
		if err != nil {
			return
		}
		defer connection.Close()
		channel, err := connection.Channel()
		if err != nil {
			return
		}
		defer channel.Close()
		topology := topologyFor(subscription)
		_, _ = channel.QueueDelete(topology.Main.Name, false, false, false)
		for _, retry := range topology.Retry {
			_, _ = channel.QueueDelete(retry.Name, false, false, false)
		}
		_, _ = channel.QueueDelete(topology.DeadLetter.Name, false, false, false)
	})
}

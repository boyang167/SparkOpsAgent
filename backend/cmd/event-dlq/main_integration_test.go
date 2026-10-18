//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
)

const replayIntegrationEventName = "test.DLQEvent"

type replayIntegrationEvent struct {
	Value string    `json:"value"`
	At    time.Time `json:"at"`
}

func (e replayIntegrationEvent) EventName() string     { return replayIntegrationEventName }
func (e replayIntegrationEvent) OccurredAt() time.Time { return e.At }

func TestReplayTargetsOnlyRequestedSubscriberAndPreservesHeaders(t *testing.T) {
	amqpURL := requireDLQIntegrationEnv(t, "AMQP_URL")
	connection, channel := openDLQIntegrationChannel(t, amqpURL)
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	targetMain := "sparkops.event." + suffix + ".target.test-dlqevent.v1"
	targetDLQ := targetMain + ".dlq"
	otherMain := "sparkops.event." + suffix + ".other.test-dlqevent.v1"
	declareReplayQueue(t, channel, targetMain)
	declareReplayQueue(t, channel, targetDLQ)
	declareReplayQueue(t, channel, otherMain)
	cleanupReplayQueues(t, connection, targetMain, targetDLQ, otherMain)

	registry := replayIntegrationRegistry(t)
	envelope, err := registry.Encode(context.Background(), replayIntegrationEvent{Value: "preserved", At: time.Now().UTC()})
	require.NoError(t, err)
	body, err := json.Marshal(envelope)
	require.NoError(t, err)
	headers := amqp.Table{"x-sparkops-attempts": int32(4), "x-safe": "kept"}
	require.NoError(t, channel.PublishWithContext(context.Background(), "", targetDLQ, false, false, amqp.Publishing{
		Headers: headers, ContentType: "application/json", DeliveryMode: amqp.Persistent,
		MessageId: envelope.ID, Body: body,
	}))

	client, err := openQueueClient(amqpURL, true)
	require.NoError(t, err)
	defer client.Close()
	require.NoError(t, replay(context.Background(), client, client, registry, targetDLQ, 1, &bytes.Buffer{}))

	target, ok, err := channel.Get(targetMain, false)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, body, target.Body)
	require.Equal(t, envelope.ID, target.MessageId)
	require.EqualValues(t, int32(4), target.Headers["x-sparkops-attempts"])
	require.Equal(t, "kept", target.Headers["x-safe"])
	require.NoError(t, target.Ack(false))
	_, ok, err = channel.Get(otherMain, true)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestReplayMandatoryReturnDoesNotAckDLQ(t *testing.T) {
	amqpURL := requireDLQIntegrationEnv(t, "AMQP_URL")
	connection, channel := openDLQIntegrationChannel(t, amqpURL)
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	missingMain := "sparkops.event." + suffix + ".missing.test-dlqevent.v1"
	dlq := missingMain + ".dlq"
	declareReplayQueue(t, channel, dlq)
	cleanupReplayQueues(t, connection, dlq)

	registry := replayIntegrationRegistry(t)
	envelope, err := registry.Encode(context.Background(), replayIntegrationEvent{Value: "return", At: time.Now().UTC()})
	require.NoError(t, err)
	body, err := json.Marshal(envelope)
	require.NoError(t, err)
	require.NoError(t, channel.PublishWithContext(context.Background(), "", dlq, false, false, amqp.Publishing{
		MessageId: envelope.ID, Body: body,
	}))

	client, err := openQueueClient(amqpURL, true)
	require.NoError(t, err)
	err = replay(context.Background(), client, client, registry, dlq, 1, &bytes.Buffer{})
	require.ErrorContains(t, err, "returned")
	require.NoError(t, client.Close())
	require.Eventually(t, func() bool {
		queue, inspectErr := channel.QueueInspect(dlq)
		return inspectErr == nil && queue.Messages == 1
	}, 3*time.Second, 20*time.Millisecond)
}

func TestRetryExhaustionDLQAttemptsSurviveCLIReplay(t *testing.T) {
	amqpURL := requireDLQIntegrationEnv(t, "AMQP_URL")
	connection, channel := openDLQIntegrationChannel(t, amqpURL)
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	subscriber := "integration." + suffix + ".retry-replay"
	mainQueue := "sparkops.event." + subscriber + ".test-dlqevent.v1"
	dlq := mainQueue + ".dlq"
	queues := declareShortReplayRetryTopology(t, channel, mainQueue, dlq)
	cleanupReplayQueues(t, connection, queues...)

	registry := replayIntegrationRegistry(t)
	envelope, err := registry.Encode(
		context.Background(),
		replayIntegrationEvent{Value: "retry-exhaustion", At: time.Now().UTC()},
	)
	require.NoError(t, err)
	transport, err := messaging.NewAMQPEventTransport(amqpURL, nil)
	require.NoError(t, err)
	consumer := messaging.NewInboxConsumer(
		directTransactor{},
		alwaysInsertInbox{},
		registry,
		transport,
		nil,
		nil,
	)
	subscription := messaging.Subscription{
		EventName:  replayIntegrationEventName,
		Version:    1,
		Subscriber: subscriber,
		Handler: func(context.Context, events.Event) error {
			return errors.New("temporary")
		},
	}
	require.NoError(t, transport.Consume(
		context.Background(),
		subscription,
		func(ctx context.Context, delivered events.Envelope, attempts int) error {
			return consumer.Handle(ctx, subscription, delivered, attempts)
		},
	))
	require.NoError(t, transport.Publish(context.Background(), envelope))
	require.Eventually(t, func() bool {
		queue, inspectErr := channel.QueueInspect(dlq)
		return inspectErr == nil && queue.Messages == 1
	}, 10*time.Second, 20*time.Millisecond)
	require.NoError(t, transport.Close())

	deadLetter, ok, err := channel.Get(dlq, false)
	require.NoError(t, err)
	require.True(t, ok)
	require.EqualValues(t, 4, deadLetter.Headers["x-sparkops-attempts"])
	require.NoError(t, deadLetter.Nack(false, true))

	client, err := openQueueClient(amqpURL, true)
	require.NoError(t, err)
	require.NoError(t, replay(context.Background(), client, client, registry, dlq, 1, &bytes.Buffer{}))
	require.NoError(t, client.Close())

	replayed, ok, err := channel.Get(mainQueue, false)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, envelope.ID, replayed.MessageId)
	require.EqualValues(t, 4, replayed.Headers["x-sparkops-attempts"])
	require.NoError(t, replayed.Ack(false))
}

type directTransactor struct{}

func (directTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type alwaysInsertInbox struct{}

func (alwaysInsertInbox) TryInsert(context.Context, string, string, string, time.Time) (bool, error) {
	return true, nil
}

func declareShortReplayRetryTopology(
	t *testing.T,
	channel *amqp.Channel,
	mainQueue string,
	dlq string,
) []string {
	t.Helper()
	require.NoError(t, channel.ExchangeDeclare("sparkops.events", "topic", true, false, false, false, nil))
	quorum := amqp.Table{"x-queue-type": "quorum"}
	_, err := channel.QueueDeclare(mainQueue, true, false, false, false, quorum)
	require.NoError(t, err)
	require.NoError(t, channel.QueueBind(mainQueue, replayIntegrationEventName+".v1", "sparkops.events", false, nil))
	queues := []string{mainQueue, dlq}
	for index, suffix := range []string{"5s", "30s", "5m", "30m"} {
		name := mainQueue + ".retry." + suffix
		_, err = channel.QueueDeclare(name, true, false, false, false, amqp.Table{
			"x-queue-type":              "quorum",
			"x-message-ttl":             int32(30 + index*20),
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": mainQueue,
		})
		require.NoError(t, err)
		queues = append(queues, name)
	}
	_, err = channel.QueueDeclare(dlq, true, false, false, false, quorum)
	require.NoError(t, err)
	return queues
}

func replayIntegrationRegistry(t *testing.T) *events.Registry {
	t.Helper()
	registry := events.NewRegistry()
	require.NoError(t, events.RegisterJSON[replayIntegrationEvent](
		registry,
		replayIntegrationEventName,
		1,
		func(replayIntegrationEvent) events.AggregateRef {
			return events.AggregateRef{Type: "test", ID: "dlq"}
		},
	))
	return registry
}

func requireDLQIntegrationEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value != "" {
		return value
	}
	if strings.EqualFold(os.Getenv("CI"), "true") || strings.EqualFold(os.Getenv("REQUIRE_INTEGRATION"), "true") {
		t.Fatalf("%s is required when integration tests run in CI", name)
	}
	t.Skipf("%s is required for integration tests", name)
	return ""
}

func openDLQIntegrationChannel(t *testing.T, amqpURL string) (*amqp.Connection, *amqp.Channel) {
	t.Helper()
	connection, err := amqp.Dial(amqpURL)
	require.NoError(t, err)
	channel, err := connection.Channel()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = channel.Close()
		_ = connection.Close()
	})
	return connection, channel
}

func declareReplayQueue(t *testing.T, channel *amqp.Channel, name string) {
	t.Helper()
	_, err := channel.QueueDeclare(name, true, false, false, false, nil)
	require.NoError(t, err)
}

func cleanupReplayQueues(t *testing.T, connection *amqp.Connection, queues ...string) {
	t.Helper()
	t.Cleanup(func() {
		channel, err := connection.Channel()
		if err != nil {
			return
		}
		defer channel.Close()
		for _, queue := range queues {
			_, _ = channel.QueueDelete(queue, false, false, false)
		}
	})
}

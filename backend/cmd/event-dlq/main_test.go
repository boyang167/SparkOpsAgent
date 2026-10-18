package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
)

func TestParseOptionsRequiresExplicitDLQQueue(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing operation", want: "operation"},
		{name: "unknown operation", args: []string{"purge", "--queue", "sparkops.event.a.b.v1.dlq"}, want: "inspect or replay"},
		{name: "missing queue", args: []string{"inspect"}, want: "--queue"},
		{name: "non dlq", args: []string{"inspect", "--queue", "sparkops.event.a.b.v1"}, want: "DLQ"},
		{name: "wildcard", args: []string{"inspect", "--queue", "sparkops.event.*.v1.dlq"}, want: "DLQ"},
		{name: "replay requires limit", args: []string{"replay", "--queue", "sparkops.event.a.b.v1.dlq"}, want: "--limit"},
		{name: "zero limit", args: []string{"replay", "--queue", "sparkops.event.a.b.v1.dlq", "--limit", "0"}, want: "positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseOptions(tt.args)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestParseOptionsAcceptsInspectAndBoundedReplay(t *testing.T) {
	inspect, err := parseOptions([]string{"inspect", "--queue", "sparkops.event.notification.build-completed.v1.dlq"})
	require.NoError(t, err)
	require.Equal(t, "inspect", inspect.operation)
	require.Zero(t, inspect.limit)

	replay, err := parseOptions([]string{
		"replay", "--queue", "sparkops.event.notification.build-completed.v1.dlq", "--limit", "100",
	})
	require.NoError(t, err)
	require.Equal(t, 100, replay.limit)
}

type recordingAcknowledger struct {
	acked  []uint64
	nacked []uint64
}

func (a *recordingAcknowledger) Ack(tag uint64, _ bool) error {
	a.acked = append(a.acked, tag)
	return nil
}

func (a *recordingAcknowledger) Nack(tag uint64, _, _ bool) error {
	a.nacked = append(a.nacked, tag)
	return nil
}

func (a *recordingAcknowledger) Reject(tag uint64, _ bool) error {
	a.nacked = append(a.nacked, tag)
	return nil
}

type fakeQueueClient struct {
	deliveries []amqp.Delivery
	index      int
	inspected  string
}

func (c *fakeQueueClient) QueueInspect(name string) (amqp.Queue, error) {
	c.inspected = name
	return amqp.Queue{Name: name, Messages: 2, Consumers: 0}, nil
}

func (c *fakeQueueClient) Get(string, bool) (amqp.Delivery, bool, error) {
	if c.index == len(c.deliveries) {
		return amqp.Delivery{}, false, nil
	}
	delivery := c.deliveries[c.index]
	c.index++
	return delivery, true, nil
}

func (c *fakeQueueClient) Close() error { return nil }

type replayedDelivery struct {
	queue   string
	body    []byte
	headers amqp.Table
	id      string
}

type fakePublisher struct {
	deliveries []replayedDelivery
	err        error
}

func (p *fakePublisher) PublishToQueue(
	_ context.Context,
	queue string,
	delivery amqp.Delivery,
) error {
	p.deliveries = append(p.deliveries, replayedDelivery{
		queue:   queue,
		body:    append([]byte(nil), delivery.Body...),
		headers: delivery.Headers,
		id:      delivery.MessageId,
	})
	return p.err
}

type dlqTestEvent struct {
	At time.Time `json:"at"`
}

func (e dlqTestEvent) EventName() string     { return "test.DLQEvent" }
func (e dlqTestEvent) OccurredAt() time.Time { return e.At }

func testRegistry(t *testing.T) *events.Registry {
	t.Helper()
	registry := events.NewRegistry()
	require.NoError(t, events.RegisterJSON[dlqTestEvent](
		registry,
		"test.DLQEvent",
		1,
		func(dlqTestEvent) events.AggregateRef { return events.AggregateRef{} },
	))
	return registry
}

func testDelivery(t *testing.T, acknowledger amqp.Acknowledger, tag uint64) amqp.Delivery {
	t.Helper()
	envelope := events.Envelope{
		ID:         uuid.NewString(),
		Type:       "test.DLQEvent",
		Version:    1,
		OccurredAt: time.Now().UTC(),
		Payload:    json.RawMessage(`{"at":"2026-10-18T00:00:00Z"}`),
	}
	body, err := json.Marshal(envelope)
	require.NoError(t, err)
	return amqp.Delivery{
		Acknowledger: acknowledger,
		DeliveryTag:  tag,
		Body:         body,
		MessageId:    envelope.ID,
		Headers:      amqp.Table{"x-sparkops-attempts": int32(3), "safe": "kept"},
	}
}

func TestReplayKeepsOriginalEventIDAndAcksOnlyAfterConfirmedPublish(t *testing.T) {
	acks := &recordingAcknowledger{}
	first := testDelivery(t, acks, 1)
	second := testDelivery(t, acks, 2)
	client := &fakeQueueClient{deliveries: []amqp.Delivery{first, second}}
	publisher := &fakePublisher{}
	var output bytes.Buffer

	err := replay(
		context.Background(),
		client,
		publisher,
		testRegistry(t),
		"sparkops.event.target.test-dlqevent.v1.dlq",
		2,
		&output,
	)

	require.NoError(t, err)
	require.Len(t, publisher.deliveries, 2)
	for index, published := range publisher.deliveries {
		original := []amqp.Delivery{first, second}[index]
		require.Equal(t, "sparkops.event.target.test-dlqevent.v1", published.queue)
		require.Equal(t, original.Body, published.body)
		require.Equal(t, original.MessageId, published.id)
		require.Equal(t, int32(3), published.headers["x-sparkops-attempts"])
		require.Equal(t, "kept", published.headers["safe"])
	}
	require.Equal(t, []uint64{1, 2}, acks.acked)
	require.Empty(t, acks.nacked)
	require.Contains(t, output.String(), "replayed=2")
}

func TestReplayDoesNotAckWhenConfirmedPublishFails(t *testing.T) {
	acks := &recordingAcknowledger{}
	delivery := testDelivery(t, acks, 1)
	client := &fakeQueueClient{deliveries: []amqp.Delivery{delivery}}
	publisher := &fakePublisher{err: errors.New("publisher confirm failed")}

	err := replay(
		context.Background(),
		client,
		publisher,
		testRegistry(t),
		"sparkops.event.target.test-dlqevent.v1.dlq",
		1,
		&bytes.Buffer{},
	)

	require.ErrorContains(t, err, "publisher confirm failed")
	require.Empty(t, acks.acked)
}

func TestReplayRejectsInvalidEnvelopeWithoutAck(t *testing.T) {
	acks := &recordingAcknowledger{}
	client := &fakeQueueClient{deliveries: []amqp.Delivery{{
		Acknowledger: acks,
		DeliveryTag:  1,
		Body:         []byte(`{"id":"not-a-uuid"}`),
	}}}

	err := replay(
		context.Background(),
		client,
		&fakePublisher{},
		testRegistry(t),
		"sparkops.event.target.test-dlqevent.v1.dlq",
		1,
		&bytes.Buffer{},
	)

	require.Error(t, err)
	require.Empty(t, acks.acked)
}

func TestReplayRejectsQueueWhoseEventIdentityDoesNotMatchEnvelope(t *testing.T) {
	acks := &recordingAcknowledger{}
	delivery := testDelivery(t, acks, 1)
	publisher := &fakePublisher{}

	err := replay(
		context.Background(),
		&fakeQueueClient{deliveries: []amqp.Delivery{delivery}},
		publisher,
		testRegistry(t),
		"sparkops.event.other.other-event.v1.dlq",
		1,
		&bytes.Buffer{},
	)

	require.ErrorContains(t, err, "does not match")
	require.Empty(t, publisher.deliveries)
	require.Empty(t, acks.acked)
}

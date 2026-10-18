//go:build integration

package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const integrationEventName = "integration.FaultWindow"

type integrationEvent struct {
	Key string    `json:"key"`
	At  time.Time `json:"at"`
}

func (e integrationEvent) EventName() string     { return integrationEventName }
func (e integrationEvent) OccurredAt() time.Time { return e.At }

type messagingIntegrationFixture struct {
	db         *gorm.DB
	transactor postgres.Transactor
	registry   *events.Registry
	outbox     persistence.OutboxStore
	inbox      persistence.InboxStore
	operations persistence.OperationStore
	amqpURL    string
	connection *amqp.Connection
	suffix     string
}

func newMessagingIntegrationFixture(t *testing.T) *messagingIntegrationFixture {
	t.Helper()
	dsn := requireIntegrationEnv(t, "DATABASE_DSN")
	amqpURL := requireIntegrationEnv(t, "AMQP_URL")
	db := openIsolatedMessagingPostgres(t, dsn)
	require.NoError(t, db.Exec(`
		CREATE TABLE integration_effects (
			event_id UUID NOT NULL,
			subscriber VARCHAR(200) NOT NULL,
			PRIMARY KEY (event_id, subscriber)
		);
		CREATE TABLE integration_business (
			id UUID PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL
		);
		CREATE TABLE integration_external_effects (
			idempotency_key VARCHAR(240) PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL
		)
	`).Error)
	connection, err := amqp.Dial(amqpURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	registry := events.NewRegistry()
	require.NoError(t, events.RegisterJSON[integrationEvent](
		registry,
		integrationEventName,
		1,
		func(ev integrationEvent) events.AggregateRef {
			return events.AggregateRef{Type: "integration", ID: ev.Key}
		},
	))
	return &messagingIntegrationFixture{
		db:         db,
		transactor: postgres.NewTransactor(db),
		registry:   registry,
		outbox:     persistence.NewOutboxStore(db),
		inbox:      persistence.NewInboxStore(db),
		operations: persistence.NewOperationStore(db),
		amqpURL:    amqpURL,
		connection: connection,
		suffix:     strings.ReplaceAll(uuid.NewString(), "-", ""),
	}
}

func (f *messagingIntegrationFixture) subscription(name string, handler Handler) Subscription {
	return Subscription{
		EventName:  integrationEventName,
		Version:    1,
		Subscriber: "integration." + f.suffix + "." + name,
		Handler:    handler,
	}
}

func (f *messagingIntegrationFixture) effectHandler(subscriber string) Handler {
	return func(ctx context.Context, _ events.Event) error {
		metadata, ok := events.MetadataFromContext(ctx)
		if !ok {
			return errors.New("event metadata missing")
		}
		return postgres.DB(ctx, f.db).Exec(`
			INSERT INTO integration_effects(event_id, subscriber)
			VALUES (?, ?)
			ON CONFLICT DO NOTHING
		`, metadata.EventID, subscriber).Error
	}
}

func (f *messagingIntegrationFixture) envelope(t *testing.T, key string) events.Envelope {
	t.Helper()
	envelope, err := f.registry.Encode(
		context.Background(),
		integrationEvent{Key: key, At: time.Now().UTC()},
	)
	require.NoError(t, err)
	return envelope
}

func (f *messagingIntegrationFixture) insertBusinessAndOutbox(t *testing.T, envelope events.Envelope) {
	t.Helper()
	require.NoError(t, f.transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		if err := postgres.DB(ctx, f.db).Exec(
			"INSERT INTO integration_business(id, created_at) VALUES (?, ?)",
			uuid.NewString(),
			time.Now().UTC(),
		).Error; err != nil {
			return err
		}
		return f.outbox.Insert(ctx, envelope)
	}))
}

func (f *messagingIntegrationFixture) transport(t *testing.T) EventTransport {
	t.Helper()
	metrics, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	transport, err := NewAMQPEventTransport(f.amqpURL, metrics)
	require.NoError(t, err)
	t.Cleanup(func() { _ = transport.Close() })
	return transport
}

func (f *messagingIntegrationFixture) cleanupTopology(t *testing.T, subscriptions ...Subscription) {
	t.Helper()
	t.Cleanup(func() {
		channel, err := f.connection.Channel()
		if err != nil {
			return
		}
		defer channel.Close()
		for _, subscription := range subscriptions {
			topology := topologyFor(subscription)
			_, _ = channel.QueueDelete(topology.Main.Name, false, false, false)
			for _, retry := range topology.Retry {
				_, _ = channel.QueueDelete(retry.Name, false, false, false)
			}
			_, _ = channel.QueueDelete(topology.DeadLetter.Name, false, false, false)
		}
	})
}

func TestEventPipelineBusinessCommitWhileBrokerUnavailableRecovers(t *testing.T) {
	f := newMessagingIntegrationFixture(t)
	subscription := f.subscription("broker-recovery", nil)
	subscription.Handler = f.effectHandler(subscription.Subscriber)
	f.cleanupTopology(t, subscription)
	envelope := f.envelope(t, "broker-recovery")
	f.insertBusinessAndOutbox(t, envelope)

	_, err := NewAMQPEventTransport("amqp://guest:guest@127.0.0.1:1/", nil)
	require.Error(t, err)
	require.Equal(t, int64(1), countRows(t, f.db, "integration_business"))
	require.Equal(t, int64(1), countWhere(t, f.db, "platform.event_outbox", "status = 'pending'"))

	transport := f.transport(t)
	metrics, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	consumer := NewInboxConsumer(f.transactor, f.inbox, f.registry, transport, nil, metrics)
	dispatcher := NewOutboxDispatcher(
		f.outbox,
		transport,
		OutboxDispatcherConfig{
			Owner: "recovery-" + f.suffix, BatchSize: 10, Lease: time.Second, PollInterval: 10 * time.Millisecond,
		},
		nil,
		metrics,
	)
	bus := NewReliableBus(f.registry, f.outbox, transport, dispatcher, consumer)
	require.NoError(t, bus.Subscribe(subscription.EventName, subscription.Subscriber, subscription.Handler))
	require.NoError(t, bus.Start(context.Background()))
	t.Cleanup(func() { _ = bus.Close() })

	require.Eventually(t, func() bool {
		return countRows(t, f.db, "integration_effects") == 1 &&
			countWhere(t, f.db, "platform.event_outbox", "status = 'published'") == 1
	}, 10*time.Second, 25*time.Millisecond)
}

func TestEventPipelineConfirmBeforeMarkCrashRepublishesWithOneInboxEffect(t *testing.T) {
	f := newMessagingIntegrationFixture(t)
	subscription := f.subscription("confirm-before-mark", nil)
	subscription.Handler = f.effectHandler(subscription.Subscriber)
	f.cleanupTopology(t, subscription)
	transport := f.transport(t)
	require.NoError(t, transport.Declare(context.Background(), []Subscription{subscription}))
	metrics, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	consumer := NewInboxConsumer(f.transactor, f.inbox, f.registry, transport, nil, metrics)
	require.NoError(t, transport.Consume(
		context.Background(),
		subscription,
		func(ctx context.Context, envelope events.Envelope, attempts int) error {
			return consumer.Handle(ctx, subscription, envelope, attempts)
		},
	))

	envelope := f.envelope(t, "confirm-before-mark")
	f.insertBusinessAndOutbox(t, envelope)
	now := time.Now().UTC().Add(5 * time.Second)
	claimed, err := f.outbox.Claim(context.Background(), "crashed", 1, now, time.Second)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.NoError(t, transport.Publish(context.Background(), claimed[0].Envelope))
	require.Eventually(t, func() bool {
		return countRows(t, f.db, "integration_effects") == 1
	}, 5*time.Second, 20*time.Millisecond)

	reclaimed, err := f.outbox.Claim(context.Background(), "recovery", 1, now.Add(2*time.Second), time.Second)
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	require.NoError(t, transport.Publish(context.Background(), reclaimed[0].Envelope))
	require.NoError(t, f.outbox.MarkPublished(context.Background(), reclaimed[0].ID, "recovery", time.Now().UTC()))
	require.Eventually(t, func() bool {
		return countRows(t, f.db, "integration_effects") == 1 &&
			countRows(t, f.db, "platform.event_inbox") == 1
	}, 5*time.Second, 20*time.Millisecond)
}

func TestEventPipelineCommitBeforeAckRedeliveryHasOneEffect(t *testing.T) {
	f := newMessagingIntegrationFixture(t)
	subscription := f.subscription("commit-before-ack", nil)
	subscription.Handler = f.effectHandler(subscription.Subscriber)
	f.cleanupTopology(t, subscription)
	transport := f.transport(t)
	require.NoError(t, transport.Declare(context.Background(), []Subscription{subscription}))
	envelope := f.envelope(t, "commit-before-ack")
	require.NoError(t, transport.Publish(context.Background(), envelope))
	channel, err := f.connection.Channel()
	require.NoError(t, err)
	t.Cleanup(func() { _ = channel.Close() })
	metrics, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	consumer := NewInboxConsumer(f.transactor, f.inbox, f.registry, transport, nil, metrics)

	first := getDeliveryEventually(t, channel, topologyFor(subscription).Main.Name)
	var decoded events.Envelope
	require.NoError(t, json.Unmarshal(first.Body, &decoded))
	require.NoError(t, consumer.Handle(context.Background(), subscription, decoded, 0))
	require.NoError(t, first.Nack(false, true))

	second := getDeliveryEventually(t, channel, topologyFor(subscription).Main.Name)
	require.NoError(t, json.Unmarshal(second.Body, &decoded))
	require.NoError(t, consumer.Handle(context.Background(), subscription, decoded, 0))
	require.NoError(t, second.Ack(false))
	require.Equal(t, int64(1), countRows(t, f.db, "integration_effects"))
	require.Equal(t, int64(1), countRows(t, f.db, "platform.event_inbox"))
}

func TestEventPipelineFanoutAndCompetingInstancesConvergeOnce(t *testing.T) {
	f := newMessagingIntegrationFixture(t)
	first := f.subscription("fanout-one", nil)
	second := f.subscription("fanout-two", nil)
	first.Handler = f.effectHandler(first.Subscriber)
	second.Handler = f.effectHandler(second.Subscriber)
	competing := f.subscription("competing", nil)
	competing.Handler = f.effectHandler(competing.Subscriber)
	f.cleanupTopology(t, first, second, competing)

	transportOne := f.transport(t)
	transportTwo := f.transport(t)
	require.NoError(t, transportOne.Declare(context.Background(), []Subscription{first, second, competing}))
	metricsOne, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	metricsTwo, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	consumerOne := NewInboxConsumer(f.transactor, f.inbox, f.registry, transportOne, nil, metricsOne)
	consumerTwo := NewInboxConsumer(f.transactor, f.inbox, f.registry, transportTwo, nil, metricsTwo)
	require.NoError(t, transportOne.Consume(context.Background(), first, func(ctx context.Context, envelope events.Envelope, attempts int) error {
		return consumerOne.Handle(ctx, first, envelope, attempts)
	}))
	require.NoError(t, transportOne.Consume(context.Background(), second, func(ctx context.Context, envelope events.Envelope, attempts int) error {
		return consumerOne.Handle(ctx, second, envelope, attempts)
	}))
	require.NoError(t, transportOne.Consume(context.Background(), competing, func(ctx context.Context, envelope events.Envelope, attempts int) error {
		return consumerOne.Handle(ctx, competing, envelope, attempts)
	}))
	require.NoError(t, transportTwo.Consume(context.Background(), competing, func(ctx context.Context, envelope events.Envelope, attempts int) error {
		return consumerTwo.Handle(ctx, competing, envelope, attempts)
	}))

	envelope := f.envelope(t, "fanout")
	require.NoError(t, transportOne.Publish(context.Background(), envelope))
	require.NoError(t, transportOne.Publish(context.Background(), envelope))
	require.Eventually(t, func() bool {
		return countRows(t, f.db, "integration_effects") == 3 &&
			countRows(t, f.db, "platform.event_inbox") == 3
	}, 10*time.Second, 20*time.Millisecond)
}

func TestEventPipelineTemporaryFailureUsesEveryRetryTier(t *testing.T) {
	f := newMessagingIntegrationFixture(t)
	var calls sync.Map
	succeeded := make(chan struct{}, 1)
	subscription := f.subscription("retry-tiers", func(_ context.Context, event events.Event) error {
		key := event.(integrationEvent).Key
		value, _ := calls.LoadOrStore(key, new(atomic.Int32))
		call := value.(*atomic.Int32).Add(1)
		if key == "eventual-success" && call == 5 {
			succeeded <- struct{}{}
			return nil
		}
		return errors.New("temporary")
	})
	f.cleanupTopology(t, subscription)
	declareShortRetryTopology(t, f.connection, subscription)
	transport := f.transport(t)
	metrics, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	consumer := NewInboxConsumer(f.transactor, f.inbox, f.registry, transport, nil, metrics)
	seen := make(map[string][]int)
	var seenMu sync.Mutex
	require.NoError(t, transport.Consume(context.Background(), subscription, func(ctx context.Context, envelope events.Envelope, attempts int) error {
		seenMu.Lock()
		seen[envelope.ID] = append(seen[envelope.ID], attempts)
		seenMu.Unlock()
		return consumer.Handle(ctx, subscription, envelope, attempts)
	}))
	successEnvelope := f.envelope(t, "eventual-success")
	deadEnvelope := f.envelope(t, "dead-letter")
	require.NoError(t, transport.Publish(context.Background(), successEnvelope))
	require.NoError(t, transport.Publish(context.Background(), deadEnvelope))

	channel, err := f.connection.Channel()
	require.NoError(t, err)
	t.Cleanup(func() { _ = channel.Close() })
	require.Eventually(t, func() bool {
		queue, inspectErr := channel.QueueInspect(topologyFor(subscription).DeadLetter.Name)
		return inspectErr == nil && queue.Messages == 1
	}, 10*time.Second, 20*time.Millisecond)
	select {
	case <-succeeded:
	case <-time.After(10 * time.Second):
		t.Fatal("message did not traverse retry TTL queues to eventual success")
	}
	deadLetter, ok, err := channel.Get(topologyFor(subscription).DeadLetter.Name, false)
	require.NoError(t, err)
	require.True(t, ok)
	require.EqualValues(t, maxEventRetryAttempts, deadLetter.Headers[eventAttemptsHeader])
	require.NoError(t, deadLetter.Ack(false))
	seenMu.Lock()
	defer seenMu.Unlock()
	require.Equal(t, []int{0, 1, 2, 3, 4}, seen[successEnvelope.ID])
	require.Equal(t, []int{0, 1, 2, 3, 4}, seen[deadEnvelope.ID])
}

func declareShortRetryTopology(t *testing.T, connection *amqp.Connection, subscription Subscription) {
	t.Helper()
	channel, err := connection.Channel()
	require.NoError(t, err)
	t.Cleanup(func() { _ = channel.Close() })
	topology := topologyFor(subscription)
	require.NoError(t, channel.ExchangeDeclare(eventExchangeName, "topic", true, false, false, false, nil))
	_, err = channel.QueueDeclare(topology.Main.Name, true, false, false, false, nil)
	require.NoError(t, err)
	require.NoError(t, channel.QueueBind(
		topology.Main.Name,
		fmt.Sprintf("%s.v%d", subscription.EventName, subscription.Version),
		eventExchangeName,
		false,
		nil,
	))
	for index, retry := range topology.Retry {
		_, err = channel.QueueDeclare(retry.Name, true, false, false, false, amqp.Table{
			"x-message-ttl":             int32(30 + index*20),
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": topology.Main.Name,
		})
		require.NoError(t, err)
	}
	_, err = channel.QueueDeclare(topology.DeadLetter.Name, true, false, false, false, nil)
	require.NoError(t, err)
}

func TestEventPipelineMalformedPayloadReachesOnlySubscriberDLQ(t *testing.T) {
	f := newMessagingIntegrationFixture(t)
	subscription := f.subscription("malformed", func(context.Context, events.Event) error { return nil })
	other := f.subscription("malformed-other", func(context.Context, events.Event) error { return nil })
	f.cleanupTopology(t, subscription, other)
	transport := f.transport(t)
	require.NoError(t, transport.Declare(context.Background(), []Subscription{subscription, other}))
	require.NoError(t, transport.Consume(context.Background(), subscription, func(context.Context, events.Envelope, int) error {
		return nil
	}))
	channel, err := f.connection.Channel()
	require.NoError(t, err)
	t.Cleanup(func() { _ = channel.Close() })
	require.NoError(t, channel.PublishWithContext(
		context.Background(),
		eventExchangeName,
		fmt.Sprintf("%s.v1", integrationEventName),
		false,
		false,
		amqp.Publishing{
			MessageId:    uuid.NewString(),
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         []byte(`{"not":"an envelope"}`),
		},
	))
	require.Eventually(t, func() bool {
		target, targetErr := channel.QueueInspect(topologyFor(subscription).DeadLetter.Name)
		otherDLQ, otherErr := channel.QueueInspect(topologyFor(other).DeadLetter.Name)
		return targetErr == nil && otherErr == nil && target.Messages == 1 && otherDLQ.Messages == 0
	}, 5*time.Second, 25*time.Millisecond)
}

func TestEventPipelineDLQReplayWithOriginalIDIsSafe(t *testing.T) {
	f := newMessagingIntegrationFixture(t)
	subscription := f.subscription("dlq-replay", nil)
	subscription.Handler = f.effectHandler(subscription.Subscriber)
	f.cleanupTopology(t, subscription)
	transport := f.transport(t)
	require.NoError(t, transport.Declare(context.Background(), []Subscription{subscription}))
	metrics, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	consumer := NewInboxConsumer(f.transactor, f.inbox, f.registry, transport, nil, metrics)
	require.NoError(t, transport.Consume(context.Background(), subscription, func(ctx context.Context, envelope events.Envelope, attempts int) error {
		return consumer.Handle(ctx, subscription, envelope, attempts)
	}))
	envelope := f.envelope(t, "dlq-replay")
	require.NoError(t, transport.PublishDeadLetter(context.Background(), subscription, envelope, "operator test", 0))
	channel, err := f.connection.Channel()
	require.NoError(t, err)
	t.Cleanup(func() { _ = channel.Close() })
	delivery := getDeliveryEventually(t, channel, topologyFor(subscription).DeadLetter.Name)
	var replayed events.Envelope
	require.NoError(t, json.Unmarshal(delivery.Body, &replayed))
	require.Equal(t, envelope.ID, replayed.ID)
	require.NoError(t, transport.Publish(context.Background(), replayed))
	require.NoError(t, delivery.Ack(false))
	require.NoError(t, transport.Publish(context.Background(), replayed))
	require.Eventually(t, func() bool {
		return countRows(t, f.db, "integration_effects") == 1 &&
			countRows(t, f.db, "platform.event_inbox") == 1
	}, 5*time.Second, 25*time.Millisecond)
}

func TestEventPipelineExpiredLeasesAreReclaimedAndFenced(t *testing.T) {
	f := newMessagingIntegrationFixture(t)
	envelope := f.envelope(t, "lease-reclaim")
	f.insertBusinessAndOutbox(t, envelope)
	operationID := uuid.NewString()
	require.NoError(t, f.transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return f.operations.Insert(ctx, persistence.OperationRecord{
			ID:             operationID,
			EventID:        envelope.ID,
			Subscriber:     "integration.lease",
			OperationType:  "integration.operation.v1",
			IdempotencyKey: envelope.ID + ":integration.lease:integration.operation.v1:target",
			Payload:        json.RawMessage(`{"target":"one"}`),
			NextAttemptAt:  time.Now().Add(-time.Minute),
			CreatedAt:      time.Now().Add(-time.Minute),
		})
	}))
	now := time.Now().UTC().Add(5 * time.Second)
	outboxFirst, err := f.outbox.Claim(context.Background(), "outbox-old", 1, now, time.Second)
	require.NoError(t, err)
	require.Len(t, outboxFirst, 1)
	operationFirst, err := f.operations.Claim(context.Background(), "operation-old", 1, now, time.Second)
	require.NoError(t, err)
	require.Len(t, operationFirst, 1)
	outboxSecond, err := f.outbox.Claim(context.Background(), "outbox-new", 1, now.Add(2*time.Second), time.Second)
	require.NoError(t, err)
	require.Len(t, outboxSecond, 1)
	operationSecond, err := f.operations.Claim(context.Background(), "operation-new", 1, now.Add(2*time.Second), time.Second)
	require.NoError(t, err)
	require.Len(t, operationSecond, 1)
	require.ErrorIs(t, f.outbox.MarkPublished(context.Background(), envelope.ID, "outbox-old", time.Now()), persistence.ErrLeaseLost)
	require.ErrorIs(t, f.operations.MarkCompleted(context.Background(), operationID, "operation-old", time.Now()), persistence.ErrLeaseLost)
	require.NoError(t, f.outbox.MarkPublished(context.Background(), envelope.ID, "outbox-new", time.Now()))
	require.NoError(t, f.operations.MarkCompleted(context.Background(), operationID, "operation-new", time.Now()))
}

func TestOperationExternalSuccessBeforeMarkCrashConvergesOnce(t *testing.T) {
	f := newMessagingIntegrationFixture(t)
	registry := NewOperationRegistry()
	require.NoError(t, registry.Register("integration.operation.v1", func(ctx context.Context, operation Operation) error {
		return f.db.WithContext(ctx).Exec(`
			INSERT INTO integration_external_effects(idempotency_key, created_at)
			VALUES (?, ?)
			ON CONFLICT DO NOTHING
		`, operation.IdempotencyKey, time.Now().UTC()).Error
	}))
	envelope := f.envelope(t, "operation-crash")
	record := persistence.OperationRecord{
		ID:             uuid.NewString(),
		EventID:        envelope.ID,
		Subscriber:     "integration.operation",
		OperationType:  "integration.operation.v1",
		IdempotencyKey: envelope.ID + ":integration.operation:integration.operation.v1:target",
		Payload:        json.RawMessage(`{"target":"one"}`),
		NextAttemptAt:  time.Now().Add(-time.Minute),
		CreatedAt:      time.Now().Add(-time.Minute),
	}
	require.NoError(t, f.transactor.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return f.operations.Insert(ctx, record)
	}))
	now := time.Now().UTC()
	claimed, err := f.operations.Claim(context.Background(), "crashed", 1, now, time.Second)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	handler, ok := registry.Handler(claimed[0].OperationType)
	require.True(t, ok)
	require.NoError(t, handler(context.Background(), Operation{
		ID: claimed[0].ID, EventID: claimed[0].EventID, Owner: claimed[0].Subscriber,
		OperationType: claimed[0].OperationType, IdempotencyKey: claimed[0].IdempotencyKey,
		Payload: claimed[0].Payload, Attempt: claimed[0].Attempts,
	}))

	dispatcher := NewOperationDispatcher(
		f.operations,
		registry,
		OperationDispatcherConfig{Owner: "recovery", BatchSize: 1, Lease: time.Second},
		fixedIntegrationClock{now: now.Add(2 * time.Second)},
		nil,
	)
	require.NoError(t, dispatcher.DispatchOnce(context.Background()))
	require.Equal(t, int64(1), countRows(t, f.db, "integration_external_effects"))
	require.Equal(t, int64(1), countWhere(t, f.db, "platform.operation_outbox", "status = 'completed'"))
}

type fixedIntegrationClock struct{ now time.Time }

func (c fixedIntegrationClock) Now() time.Time { return c.now }

type gatedTransport struct {
	EventTransport
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (t *gatedTransport) Publish(ctx context.Context, envelope events.Envelope) error {
	t.once.Do(func() { close(t.started) })
	select {
	case <-t.release:
		return t.EventTransport.Publish(ctx, envelope)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestEventPipelineShutdownStopsClaimsAndDrainsInflightPublish(t *testing.T) {
	f := newMessagingIntegrationFixture(t)
	subscription := f.subscription("shutdown", func(context.Context, events.Event) error { return nil })
	f.cleanupTopology(t, subscription)
	realTransport := f.transport(t)
	require.NoError(t, realTransport.Declare(context.Background(), []Subscription{subscription}))
	gated := &gatedTransport{
		EventTransport: realTransport,
		started:        make(chan struct{}),
		release:        make(chan struct{}),
	}
	metrics, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	dispatcher := NewOutboxDispatcher(
		f.outbox,
		gated,
		OutboxDispatcherConfig{
			Owner: "shutdown-" + f.suffix, BatchSize: 1, Lease: time.Second, PollInterval: 10 * time.Millisecond,
		},
		nil,
		metrics,
	)
	f.insertBusinessAndOutbox(t, f.envelope(t, "first"))
	require.NoError(t, dispatcher.Start(context.Background()))
	select {
	case <-gated.started:
	case <-time.After(5 * time.Second):
		t.Fatal("dispatcher did not start publishing")
	}
	closed := make(chan error, 1)
	go func() { closed <- dispatcher.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("close returned before in-flight publish drained: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(gated.release)
	require.NoError(t, <-closed)
	f.insertBusinessAndOutbox(t, f.envelope(t, "second"))
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, int64(1), countWhere(t, f.db, "platform.event_outbox", "status = 'pending'"))
	require.Equal(t, int64(1), countWhere(t, f.db, "platform.event_outbox", "status = 'published'"))
}

func TestEventPipelineConcurrentClaimsAreDisjoint(t *testing.T) {
	f := newMessagingIntegrationFixture(t)
	for index := 0; index < 10; index++ {
		f.insertBusinessAndOutbox(t, f.envelope(t, fmt.Sprintf("claim-%d", index)))
	}
	var first, second []persistence.OutboxRecord
	var firstErr, secondErr error
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		first, firstErr = f.outbox.Claim(context.Background(), "instance-one", 5, time.Now().UTC().Add(5*time.Second), time.Minute)
	}()
	go func() {
		defer wait.Done()
		second, secondErr = f.outbox.Claim(context.Background(), "instance-two", 5, time.Now().UTC().Add(5*time.Second), time.Minute)
	}()
	wait.Wait()
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.Len(t, first, 5)
	require.Len(t, second, 5)
	seen := make(map[string]struct{}, 10)
	for _, record := range append(first, second...) {
		seen[record.ID] = struct{}{}
	}
	require.Len(t, seen, 10)
}

func openIsolatedMessagingPostgres(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	admin, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	databaseName := "sparkops_event_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE DATABASE "+databaseName).Error)
	db, err := gorm.Open(gormpostgres.Open(integrationDSNWithDatabase(t, dsn, databaseName)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	var serverVersion string
	require.NoError(t, db.Raw("SHOW server_version").Scan(&serverVersion).Error)
	t.Logf("PostgreSQL server version: %s", serverVersion)
	migration, err := os.ReadFile("../../../migrations/000070_transactional_events.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration)).Error)
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
		_ = admin.Exec(fmt.Sprintf(
			"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s'",
			databaseName,
		)).Error
		_ = admin.Exec("DROP DATABASE IF EXISTS " + databaseName).Error
		adminSQL, adminErr := admin.DB()
		if adminErr == nil {
			_ = adminSQL.Close()
		}
	})
	return db
}

func integrationDSNWithDatabase(t *testing.T, dsn, databaseName string) string {
	t.Helper()
	if strings.Contains(dsn, "://") {
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		parsed.Path = "/" + databaseName
		return parsed.String()
	}
	databaseSetting := regexp.MustCompile(`(?i)\b(?:dbname|database)\s*=\s*(?:'[^']*'|\S+)`)
	if databaseSetting.MatchString(dsn) {
		return databaseSetting.ReplaceAllString(dsn, "dbname="+databaseName)
	}
	return dsn + " dbname=" + databaseName
}

func getDeliveryEventually(t *testing.T, channel *amqp.Channel, queue string) amqp.Delivery {
	t.Helper()
	var delivery amqp.Delivery
	require.Eventually(t, func() bool {
		item, ok, err := channel.Get(queue, false)
		if err != nil || !ok {
			return false
		}
		delivery = item
		return true
	}, 5*time.Second, 20*time.Millisecond)
	return delivery
}

func countRows(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	return countWhere(t, db, table, "TRUE")
}

func countWhere(t *testing.T, db *gorm.DB, table, where string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Where(where).Count(&count).Error)
	return count
}

package messaging

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
)

type transactionStateKey struct{}

type fakeTransaction struct {
	commit   []func()
	rollback []func()
}

type recordingTransactor struct {
	commits   int
	rollbacks int
}

func (t *recordingTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	tx := &fakeTransaction{}
	err := fn(context.WithValue(ctx, transactionStateKey{}, tx))
	if err != nil {
		t.rollbacks++
		for _, rollback := range tx.rollback {
			rollback()
		}
		return err
	}
	t.commits++
	for _, commit := range tx.commit {
		commit()
	}
	return nil
}

var _ postgres.Transactor = (*recordingTransactor)(nil)

type transactionalInbox struct {
	processed map[string]bool
}

func (i *transactionalInbox) TryInsert(ctx context.Context, eventID, subscriber, _ string, _ time.Time) (bool, error) {
	key := eventID + ":" + subscriber
	if i.processed[key] {
		return false, nil
	}
	tx := ctx.Value(transactionStateKey{}).(*fakeTransaction)
	tx.commit = append(tx.commit, func() { i.processed[key] = true })
	return true, nil
}

type inboxTestEvent struct {
	events.BaseEvent
	Value string `json:"value"`
}

func TestInboxConsumerCommitsHandlerAndInboxTogether(t *testing.T) {
	consumer, tx, inbox, transport, envelope := newInboxTestConsumer(t)
	sideEffect := false
	subscription := inboxSubscription(func(ctx context.Context, _ events.Event) error {
		require.True(t, IsInboxConsumerContext(ctx))
		transaction := ctx.Value(transactionStateKey{}).(*fakeTransaction)
		transaction.commit = append(transaction.commit, func() { sideEffect = true })
		return nil
	})

	err := consumer.Handle(context.Background(), subscription, envelope, 0)

	require.NoError(t, err)
	require.Equal(t, 1, tx.commits)
	require.True(t, sideEffect)
	require.True(t, inbox.processed[envelope.ID+":"+subscription.Subscriber])
	require.Empty(t, transport.retries)
	require.Empty(t, transport.deadLetters)
}

func TestInboxConsumerSkipsHandlerForDuplicateDelivery(t *testing.T) {
	consumer, tx, inbox, _, envelope := newInboxTestConsumer(t)
	subscription := inboxSubscription(func(context.Context, events.Event) error {
		t.Fatal("duplicate delivery invoked handler")
		return nil
	})
	inbox.processed[envelope.ID+":"+subscription.Subscriber] = true

	err := consumer.Handle(context.Background(), subscription, envelope, 0)

	require.NoError(t, err)
	require.Equal(t, 1, tx.commits)
}

func TestInboxConsumerRollsBackInboxWhenHandlerFails(t *testing.T) {
	consumer, tx, inbox, transport, envelope := newInboxTestConsumer(t)
	subscription := inboxSubscription(func(context.Context, events.Event) error {
		return errors.New("database temporarily unavailable")
	})

	err := consumer.Handle(context.Background(), subscription, envelope, 0)

	require.NoError(t, err, "confirmed retry should ACK the original delivery")
	require.Equal(t, 1, tx.rollbacks)
	require.False(t, inbox.processed[envelope.ID+":"+subscription.Subscriber])
	require.Equal(t, []RetryDelay{RetryDelay(5 * time.Second)}, transport.retries)
}

func TestInboxConsumerRoutesPermanentDecodeErrorToDLQ(t *testing.T) {
	consumer, tx, inbox, transport, envelope := newInboxTestConsumer(t)
	envelope.Payload = []byte(`{"value":`)
	subscription := inboxSubscription(func(context.Context, events.Event) error {
		t.Fatal("decode failure invoked handler")
		return nil
	})

	err := consumer.Handle(context.Background(), subscription, envelope, 4)

	require.NoError(t, err, "confirmed DLQ publish should ACK the original delivery")
	require.Equal(t, 1, tx.rollbacks)
	require.False(t, inbox.processed[envelope.ID+":"+subscription.Subscriber])
	require.Len(t, transport.deadLetters, 1)
	require.Equal(t, []int{4}, transport.deadLetterAttempts)
	require.Empty(t, transport.retries)
}

func TestInboxConsumerRetriesTemporaryErrorForeverAtLastBackoffTier(t *testing.T) {
	consumer, _, _, transport, envelope := newInboxTestConsumer(t)
	subscription := inboxSubscription(func(context.Context, events.Event) error {
		return errors.New("temporary")
	})
	expected := []RetryDelay{
		RetryDelay(5 * time.Second),
		RetryDelay(30 * time.Second),
		RetryDelay(5 * time.Minute),
		RetryDelay(30 * time.Minute),
		RetryDelay(30 * time.Minute),
		RetryDelay(30 * time.Minute),
	}

	for attempts := range expected {
		require.NoError(t, consumer.Handle(context.Background(), subscription, envelope, attempts))
	}

	require.Equal(t, expected, transport.retries)
	require.Equal(t, []int{1, 2, 3, 4, 5, 6}, transport.retryAttempts)
	require.Empty(t, transport.deadLetters)
}

func TestInboxConsumerSaturatesAttemptsAcrossConsecutiveRetries(t *testing.T) {
	consumer, _, _, transport, envelope := newInboxTestConsumer(t)
	subscription := inboxSubscription(func(context.Context, events.Event) error {
		return errors.New("temporary")
	})

	require.NoError(t, consumer.Handle(context.Background(), subscription, envelope, math.MaxInt32))
	require.NoError(t, consumer.Handle(context.Background(), subscription, envelope, math.MaxInt32))

	require.Equal(t, []RetryDelay{
		RetryDelay(30 * time.Minute),
		RetryDelay(30 * time.Minute),
	}, transport.retries)
	require.Equal(t, []int{math.MaxInt32, math.MaxInt32}, transport.retryAttempts)
}

func TestInboxConsumerDoesNotAckWhenRetryPublishIsUnconfirmed(t *testing.T) {
	consumer, _, _, transport, envelope := newInboxTestConsumer(t)
	transport.publishErr = errors.New("retry confirm failed")
	subscription := inboxSubscription(func(context.Context, events.Event) error {
		return errors.New("temporary")
	})

	err := consumer.Handle(context.Background(), subscription, envelope, 0)

	require.ErrorContains(t, err, "retry confirm failed")
}

func TestInboxConsumerRecordsAllConsumptionOutcomes(t *testing.T) {
	base, tx, inbox, transport, envelope := newInboxTestConsumer(t)
	registry := prometheus.NewRegistry()
	metrics, err := NewMetrics(registry)
	require.NoError(t, err)
	consumer := NewInboxConsumer(
		tx, inbox, base.registry, transport, fixedClock{now: envelope.OccurredAt}, metrics,
	)
	success := inboxSubscription(func(context.Context, events.Event) error { return nil })

	require.NoError(t, consumer.Handle(context.Background(), success, envelope, 0))
	require.NoError(t, consumer.Handle(context.Background(), success, envelope, 0))
	permanentEnvelope := envelope
	permanentEnvelope.ID = "permanent-event"
	permanentEnvelope.Payload = []byte(`{"value":`)
	require.NoError(t, consumer.Handle(context.Background(), success, permanentEnvelope, 0))
	retryEnvelope := envelope
	retryEnvelope.ID = "retry-event"
	temporary := inboxSubscription(func(context.Context, events.Event) error {
		return errors.New("temporary")
	})
	require.NoError(t, consumer.Handle(context.Background(), temporary, retryEnvelope, 0))

	require.Equal(t, float64(1), prometheusMetricValue(t, metrics.EventConsume.WithLabelValues("test.subscriber", "success")))
	require.Equal(t, float64(1), prometheusMetricValue(t, metrics.EventConsume.WithLabelValues("test.subscriber", "duplicate")))
	require.Equal(t, float64(1), prometheusMetricValue(t, metrics.EventConsume.WithLabelValues("test.subscriber", "permanent_error")))
	require.Equal(t, float64(1), prometheusMetricValue(t, metrics.EventConsume.WithLabelValues("test.subscriber", "retry")))
	require.Equal(t, float64(1), prometheusMetricValue(t, metrics.InboxDuplicate.WithLabelValues("test.subscriber")))
	require.Equal(t, float64(1), prometheusMetricValue(t, metrics.EventDLQ.WithLabelValues("test.subscriber")))
	require.Equal(t, float64(1), prometheusMetricValue(t, metrics.EventRetry.WithLabelValues("test.subscriber", "5s")))
}

func TestInboxConsumerRecordsRetryAndDLQPublishFailures(t *testing.T) {
	base, tx, inbox, transport, envelope := newInboxTestConsumer(t)
	registry := prometheus.NewRegistry()
	metrics, err := NewMetrics(registry)
	require.NoError(t, err)
	consumer := NewInboxConsumer(
		tx, inbox, base.registry, transport, fixedClock{now: envelope.OccurredAt}, metrics,
	)
	transport.publishErr = errors.New("confirm failed")
	permanentEnvelope := envelope
	permanentEnvelope.ID = "permanent-failure"
	permanentEnvelope.Payload = []byte(`{"value":`)
	subscription := inboxSubscription(func(context.Context, events.Event) error {
		return errors.New("temporary")
	})
	require.Error(t, consumer.Handle(context.Background(), subscription, permanentEnvelope, 0))
	retryEnvelope := envelope
	retryEnvelope.ID = "retry-failure"
	require.Error(t, consumer.Handle(context.Background(), subscription, retryEnvelope, 0))

	require.Equal(t, float64(1), prometheusMetricValue(t, metrics.EventConsume.WithLabelValues("test.subscriber", "dlq_publish_failure")))
	require.Equal(t, float64(1), prometheusMetricValue(t, metrics.EventConsume.WithLabelValues("test.subscriber", "retry_publish_failure")))
	require.Zero(t, prometheusMetricValue(t, metrics.EventDLQ.WithLabelValues("test.subscriber")))
	require.Zero(t, prometheusMetricValue(t, metrics.EventRetry.WithLabelValues("test.subscriber", "5s")))
}

func TestPermanentClassificationPreservesWrappedErrors(t *testing.T) {
	cause := errors.New("invalid event")
	err := Permanent(cause)

	require.True(t, IsPermanent(err))
	require.ErrorIs(t, err, cause)
	require.False(t, IsPermanent(cause))
}

func newInboxTestConsumer(t *testing.T) (*InboxConsumer, *recordingTransactor, *transactionalInbox, *fakeEventTransport, events.Envelope) {
	t.Helper()
	registry := events.NewRegistry()
	require.NoError(t, events.RegisterJSON(registry, "test.event", 1, func(inboxTestEvent) events.AggregateRef {
		return events.AggregateRef{}
	}))
	event := inboxTestEvent{BaseEvent: events.BaseEvent{Name: "test.event", At: time.Now().UTC()}, Value: "value"}
	envelope, err := registry.Encode(context.Background(), event)
	require.NoError(t, err)
	tx := &recordingTransactor{}
	inbox := &transactionalInbox{processed: make(map[string]bool)}
	transport := &fakeEventTransport{}
	consumer := NewInboxConsumer(tx, inbox, registry, transport, fixedClock{now: event.At}, nil)
	return consumer, tx, inbox, transport, envelope
}

func inboxSubscription(handler Handler) Subscription {
	return Subscription{
		EventName: "test.event", Version: 1, Subscriber: "test.subscriber", Handler: handler,
	}
}

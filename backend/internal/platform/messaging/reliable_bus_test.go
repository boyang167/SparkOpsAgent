package messaging

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type transactionCheckingOutbox struct {
	envelopes []events.Envelope
}

func (s *transactionCheckingOutbox) Insert(ctx context.Context, envelope events.Envelope) error {
	if _, err := postgres.RequireTx(ctx); err != nil {
		return err
	}
	s.envelopes = append(s.envelopes, envelope)
	return nil
}

func (*transactionCheckingOutbox) Claim(context.Context, string, int, time.Time, time.Duration) ([]persistence.OutboxRecord, error) {
	return nil, nil
}
func (*transactionCheckingOutbox) MarkPublished(context.Context, string, string, time.Time) error {
	return nil
}
func (*transactionCheckingOutbox) Reschedule(context.Context, string, string, string, time.Time) error {
	return nil
}
func (*transactionCheckingOutbox) Stats(context.Context, time.Time) (persistence.OutboxStats, error) {
	return persistence.OutboxStats{}, nil
}

type fakeDispatcherLifecycle struct {
	started int
	closed  int
}

func (d *fakeDispatcherLifecycle) Start(context.Context) error { d.started++; return nil }
func (d *fakeDispatcherLifecycle) Close() error                { d.closed++; return nil }

func TestReliableBusRequiresTransactionAndOnlyWritesOutbox(t *testing.T) {
	registry := events.NewRegistry()
	aggregateVersion := int64(2)
	require.NoError(t, events.RegisterJSON(registry, "test.event", 1, func(inboxTestEvent) events.AggregateRef {
		return events.AggregateRef{Type: "test", ID: "aggregate-1", Version: &aggregateVersion}
	}))
	store := &transactionCheckingOutbox{}
	transport := &fakeEventTransport{}
	bus := NewReliableBus(registry, store, transport, nil, nil)
	event := inboxTestEvent{
		BaseEvent: events.BaseEvent{Name: "test.event", At: time.Now().UTC()},
		Value:     "payload",
	}

	err := bus.Publish(context.Background(), event)
	require.ErrorContains(t, err, "transaction required")

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	tx := postgres.NewTransactor(db)
	require.NoError(t, tx.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return bus.Publish(ctx, event)
	}))

	require.Len(t, store.envelopes, 1)
	require.Equal(t, "test.event", store.envelopes[0].Type)
	require.Equal(t, "aggregate-1", store.envelopes[0].AggregateID)
	require.Empty(t, transport.published, "transactional publish must not perform network I/O")
}

func TestReliableBusStartDeclaresBeforeConsumersAndDispatcher(t *testing.T) {
	registry := events.NewRegistry()
	store := &transactionCheckingOutbox{}
	transport := &orderedTransport{}
	dispatcher := &orderedDispatcher{order: &transport.order}
	consumer := &InboxConsumer{}
	bus := NewReliableBus(registry, store, transport, dispatcher, consumer)
	require.NoError(t, bus.Subscribe("test.event", "test.subscriber", func(context.Context, events.Event) error {
		return nil
	}))

	require.NoError(t, bus.Start(context.Background()))

	require.Equal(t, []string{"declare", "consume", "dispatcher"}, transport.order)
}

func TestReliableBusRejectsInvalidAndDuplicateSubscriptions(t *testing.T) {
	bus := NewReliableBus(events.NewRegistry(), &transactionCheckingOutbox{}, &fakeEventTransport{}, nil, nil)
	handler := func(context.Context, events.Event) error { return nil }

	require.Error(t, bus.Subscribe("", "test.subscriber", handler))
	require.Error(t, bus.Subscribe("test.event", "", handler))
	require.Error(t, bus.Subscribe("test.event", "test.subscriber", nil))
	require.NoError(t, bus.Subscribe("test.event", "test.subscriber", handler))
	require.ErrorContains(t, bus.Subscribe("test.event", "test.subscriber", handler), "duplicate")
}

func TestReliableBusCloseIsIdempotent(t *testing.T) {
	transport := &fakeEventTransport{}
	dispatcher := &fakeDispatcherLifecycle{}
	bus := NewReliableBus(events.NewRegistry(), &transactionCheckingOutbox{}, transport, dispatcher, &InboxConsumer{})

	require.NoError(t, bus.Close())
	require.NoError(t, bus.Close())

	require.Equal(t, 1, dispatcher.closed)
	require.Equal(t, 1, transport.closed)
}

func TestReliableBusCloseRejectsLateHandlersAndWaitsForActiveHandlers(t *testing.T) {
	registry := events.NewRegistry()
	require.NoError(t, events.RegisterJSON(registry, "test.event", 1, func(inboxTestEvent) events.AggregateRef {
		return events.AggregateRef{}
	}))
	envelope, err := registry.Encode(context.Background(), inboxTestEvent{
		BaseEvent: events.BaseEvent{Name: "test.event", At: time.Now().UTC()},
	})
	require.NoError(t, err)
	tx := &recordingTransactor{}
	inbox := &transactionalInbox{processed: make(map[string]bool)}
	transport := &lateDeliveryTransport{
		fakeEventTransport: fakeEventTransport{},
		envelope:           envelope,
		releaseHandler:     make(chan struct{}),
		handlerStarted:     make(chan struct{}),
	}
	consumer := NewInboxConsumer(
		tx, inbox, registry, transport, fixedClock{now: envelope.OccurredAt}, nil,
	)
	bus := NewReliableBus(registry, &transactionCheckingOutbox{}, transport, nil, consumer)
	var startedOnce sync.Once
	require.NoError(t, bus.Subscribe("test.event", "test.subscriber", func(context.Context, events.Event) error {
		startedOnce.Do(func() { close(transport.handlerStarted) })
		<-transport.releaseHandler
		return nil
	}))
	require.NoError(t, bus.Start(context.Background()))
	go func() { _ = transport.deliver() }()
	<-transport.handlerStarted

	closed := make(chan struct{})
	go func() {
		_ = bus.Close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("Close returned while an active handler transaction was blocked")
	case <-time.After(20 * time.Millisecond):
	}
	close(transport.releaseHandler)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close did not return after active handler completed")
	}

	require.ErrorIs(t, transport.deliver(), context.Canceled)
}

func TestReliableBusClosePreservesActiveHandlerTransactionContext(t *testing.T) {
	registry := events.NewRegistry()
	require.NoError(t, events.RegisterJSON(registry, "test.event", 1, func(inboxTestEvent) events.AggregateRef {
		return events.AggregateRef{}
	}))
	envelope, err := registry.Encode(context.Background(), inboxTestEvent{
		BaseEvent: events.BaseEvent{Name: "test.event", At: time.Now().UTC()},
	})
	require.NoError(t, err)
	tx := &recordingTransactor{}
	inbox := &transactionalInbox{processed: make(map[string]bool)}
	transport := &contextDeliveryTransport{
		fakeEventTransport: fakeEventTransport{},
		envelope:           envelope,
	}
	consumer := NewInboxConsumer(
		tx, inbox, registry, transport, fixedClock{now: envelope.OccurredAt}, nil,
	)
	bus := NewReliableBus(registry, &transactionCheckingOutbox{}, transport, nil, consumer)
	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	var startedOnce sync.Once
	require.NoError(t, bus.Subscribe("test.event", "test.subscriber", func(ctx context.Context, _ events.Event) error {
		startedOnce.Do(func() { close(handlerStarted) })
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-releaseHandler:
			return nil
		}
	}))
	require.NoError(t, bus.Start(context.Background()))
	deliveryResult := make(chan error, 1)
	go func() { deliveryResult <- transport.deliver() }()
	<-handlerStarted

	closed := make(chan error, 1)
	go func() { closed <- bus.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("Close returned before active transaction completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	require.ErrorIs(t, transport.intakeContext.Err(), context.Canceled)
	close(releaseHandler)
	require.NoError(t, <-deliveryResult)
	require.NoError(t, <-closed)
	require.Equal(t, 1, tx.commits)
	require.Zero(t, tx.rollbacks)
}

func TestReliableBusStartFailureDrainsStartedHandlerAndTerminatesLifecycle(t *testing.T) {
	registry := events.NewRegistry()
	require.NoError(t, events.RegisterJSON(registry, "test.event", 1, func(inboxTestEvent) events.AggregateRef {
		return events.AggregateRef{}
	}))
	envelope, err := registry.Encode(context.Background(), inboxTestEvent{
		BaseEvent: events.BaseEvent{Name: "test.event", At: time.Now().UTC()},
	})
	require.NoError(t, err)
	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	transport := &partialStartTransport{
		fakeEventTransport: fakeEventTransport{},
		envelope:           envelope,
		handlerStarted:     handlerStarted,
		handlerResult:      make(chan error, 1),
	}
	tx := &recordingTransactor{}
	inbox := &transactionalInbox{processed: make(map[string]bool)}
	metrics, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	consumer := NewInboxConsumer(
		tx, inbox, registry, transport, fixedClock{now: envelope.OccurredAt}, metrics,
	)
	dispatcher := &fakeDispatcherLifecycle{}
	bus := NewReliableBus(
		registry, &transactionCheckingOutbox{}, transport, dispatcher, consumer,
	)
	require.NoError(t, bus.Subscribe("test.event", "first.subscriber", func(
		context.Context,
		events.Event,
	) error {
		close(handlerStarted)
		<-releaseHandler
		return nil
	}))
	require.NoError(t, bus.Subscribe("second.event", "second.subscriber", func(
		context.Context,
		events.Event,
	) error {
		return nil
	}))

	startResult := make(chan error, 1)
	go func() { startResult <- bus.Start(context.Background()) }()
	<-handlerStarted
	select {
	case err := <-startResult:
		t.Fatalf("Start returned before the active handler drained: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(releaseHandler)
	require.NoError(t, <-transport.handlerResult)
	require.ErrorContains(t, <-startResult, "start second consumer")
	require.Equal(t, 1, transport.closed)
	require.Equal(t, 1, dispatcher.closed)

	consumeCalls := transport.consumeCalls
	require.ErrorContains(t, bus.Start(context.Background()), "closed")
	require.Equal(t, consumeCalls, transport.consumeCalls)
	require.NoError(t, bus.Close())
	require.Equal(t, 1, transport.closed)
	require.Equal(t, 1, dispatcher.closed)
}

type orderedTransport struct {
	fakeEventTransport
	order []string
}

func (t *orderedTransport) Declare(_ context.Context, subscriptions []Subscription) error {
	t.order = append(t.order, "declare")
	t.declared = append([]Subscription(nil), subscriptions...)
	return nil
}

func (t *orderedTransport) Consume(_ context.Context, subscription Subscription, _ DeliveryHandler) error {
	t.order = append(t.order, "consume")
	t.consumed = append(t.consumed, subscription)
	return nil
}

type orderedDispatcher struct {
	order *[]string
}

func (d *orderedDispatcher) Start(context.Context) error {
	*d.order = append(*d.order, "dispatcher")
	return nil
}
func (*orderedDispatcher) Close() error { return nil }

type lateDeliveryTransport struct {
	fakeEventTransport
	mu             sync.Mutex
	handler        DeliveryHandler
	envelope       events.Envelope
	releaseHandler chan struct{}
	handlerStarted chan struct{}
}

func (t *lateDeliveryTransport) Consume(_ context.Context, subscription Subscription, handler DeliveryHandler) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.consumed = append(t.consumed, subscription)
	t.handler = handler
	return nil
}

func (t *lateDeliveryTransport) deliver() error {
	t.mu.Lock()
	handler := t.handler
	t.mu.Unlock()
	return handler(context.Background(), t.envelope, 0)
}

type contextDeliveryTransport struct {
	fakeEventTransport
	handler       DeliveryHandler
	envelope      events.Envelope
	intakeContext context.Context
}

func (t *contextDeliveryTransport) Consume(
	ctx context.Context,
	subscription Subscription,
	handler DeliveryHandler,
) error {
	t.consumed = append(t.consumed, subscription)
	t.intakeContext = ctx
	t.handler = handler
	return nil
}

func (t *contextDeliveryTransport) deliver() error {
	return t.handler(t.intakeContext, t.envelope, 0)
}

type partialStartTransport struct {
	fakeEventTransport
	envelope       events.Envelope
	handlerStarted <-chan struct{}
	handlerResult  chan error
	consumeCalls   int
}

func (t *partialStartTransport) Consume(
	ctx context.Context,
	subscription Subscription,
	handler DeliveryHandler,
) error {
	t.consumeCalls++
	if t.consumeCalls == 1 {
		go func() {
			t.handlerResult <- handler(ctx, t.envelope, 0)
		}()
		return nil
	}
	<-t.handlerStarted
	return errors.New("start second consumer")
}

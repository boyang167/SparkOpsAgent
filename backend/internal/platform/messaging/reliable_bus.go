package messaging

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

var stableSubscriberName = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)

type ReliableBus struct {
	registry   *events.Registry
	outbox     persistence.OutboxStore
	transport  EventTransport
	dispatcher DispatcherLifecycle
	consumer   *InboxConsumer

	mu            sync.Mutex
	subscriptions []Subscription
	started       bool
	closed        bool
	cancel        context.CancelFunc
}

func NewReliableBus(
	registry *events.Registry,
	outbox persistence.OutboxStore,
	transport EventTransport,
	dispatcher DispatcherLifecycle,
	consumer *InboxConsumer,
) *ReliableBus {
	return &ReliableBus{
		registry: registry, outbox: outbox, transport: transport,
		dispatcher: dispatcher, consumer: consumer,
	}
}

func (b *ReliableBus) Publish(ctx context.Context, event ...events.Event) error {
	if _, err := postgres.RequireTx(ctx); err != nil {
		return err
	}
	if b.registry == nil || b.outbox == nil {
		return fmt.Errorf("reliable bus registry and outbox are required")
	}
	for _, item := range event {
		envelope, err := b.registry.Encode(ctx, item)
		if err != nil {
			return err
		}
		if err := b.outbox.Insert(ctx, envelope); err != nil {
			return err
		}
	}
	return nil
}

func (b *ReliableBus) Subscribe(eventName, subscriber string, handler Handler) error {
	eventName = strings.TrimSpace(eventName)
	subscriber = strings.TrimSpace(subscriber)
	if eventName == "" {
		return fmt.Errorf("event name is required")
	}
	if !stableSubscriberName.MatchString(subscriber) {
		return fmt.Errorf("stable subscriber name is required")
	}
	if handler == nil {
		return fmt.Errorf("event handler is required")
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.started {
		return fmt.Errorf("subscriptions cannot be added after reliable bus start")
	}
	for _, registered := range b.subscriptions {
		if registered.EventName == eventName && registered.Subscriber == subscriber {
			return fmt.Errorf("duplicate subscription for event %q and subscriber %q", eventName, subscriber)
		}
	}
	b.subscriptions = append(b.subscriptions, Subscription{
		EventName: eventName, Version: 1, Subscriber: subscriber, Handler: handler,
	})
	return nil
}

func (b *ReliableBus) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return fmt.Errorf("reliable bus is closed")
	}
	if b.started {
		b.mu.Unlock()
		return nil
	}
	if b.transport == nil {
		b.mu.Unlock()
		return fmt.Errorf("event transport is required")
	}
	subscriptions := append([]Subscription(nil), b.subscriptions...)
	runCtx, cancel := context.WithCancel(ctx)
	b.cancel = cancel
	b.started = true
	b.mu.Unlock()

	if err := validateEventTopologies(subscriptions); err != nil {
		return b.abortStart(cancel, err)
	}
	if err := b.transport.Declare(runCtx, subscriptions); err != nil {
		return b.abortStart(cancel, err)
	}
	for _, subscription := range subscriptions {
		if b.consumer == nil {
			return b.abortStart(cancel, fmt.Errorf("inbox consumer is required"))
		}
		current := subscription
		if err := b.transport.Consume(runCtx, current, func(
			deliveryCtx context.Context,
			envelope events.Envelope,
			attempts int,
		) error {
			handlerCtx := context.WithoutCancel(deliveryCtx)
			return b.consumer.Handle(handlerCtx, current, envelope, attempts)
		}); err != nil {
			return b.abortStart(cancel, err)
		}
	}
	if b.dispatcher != nil {
		if err := b.dispatcher.Start(runCtx); err != nil {
			return b.abortStart(cancel, err)
		}
	}
	return nil
}

func (b *ReliableBus) abortStart(cancel context.CancelFunc, startErr error) error {
	b.mu.Lock()
	b.closed = true
	b.cancel = nil
	b.mu.Unlock()

	failures := []error{startErr}
	if b.consumer != nil {
		b.consumer.StopAccepting()
	}
	cancel()
	if b.dispatcher != nil {
		if err := b.dispatcher.Close(); err != nil {
			failures = append(failures, fmt.Errorf("close dispatcher after start failure: %w", err))
		}
	}
	if b.consumer != nil {
		b.consumer.Wait()
	}
	if b.transport != nil {
		if err := b.transport.Close(); err != nil {
			failures = append(failures, fmt.Errorf("close transport after start failure: %w", err))
		}
	}
	return errors.Join(failures...)
}

func (b *ReliableBus) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	cancel := b.cancel
	b.cancel = nil
	b.mu.Unlock()

	var closeErrors []error
	if b.dispatcher != nil {
		if err := b.dispatcher.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	if b.consumer != nil {
		b.consumer.StopAccepting()
	}
	if cancel != nil {
		cancel()
	}
	if b.consumer != nil {
		b.consumer.Wait()
	}
	if b.transport != nil {
		if err := b.transport.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

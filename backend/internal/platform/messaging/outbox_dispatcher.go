package messaging

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
)

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type OutboxDispatcherConfig struct {
	Owner        string
	BatchSize    int
	Lease        time.Duration
	PollInterval time.Duration
}

type DispatcherLifecycle interface {
	Start(context.Context) error
	Close() error
}

type OutboxDispatcher struct {
	store     persistence.OutboxStore
	transport EventTransport
	config    OutboxDispatcherConfig
	clock     Clock

	mu      sync.Mutex
	started bool
	closed  bool
	stop    chan struct{}
	done    chan struct{}
}

func NewOutboxDispatcher(
	store persistence.OutboxStore,
	transport EventTransport,
	config OutboxDispatcherConfig,
	clock Clock,
) *OutboxDispatcher {
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.Lease <= 0 {
		config.Lease = time.Minute
	}
	if config.PollInterval <= 0 {
		config.PollInterval = time.Second
	}
	if clock == nil {
		clock = systemClock{}
	}
	return &OutboxDispatcher{store: store, transport: transport, config: config, clock: clock}
}

func (d *OutboxDispatcher) DispatchOnce(ctx context.Context) error {
	if d.store == nil || d.transport == nil {
		return fmt.Errorf("outbox dispatcher dependencies are required")
	}
	if d.config.Owner == "" {
		return fmt.Errorf("outbox dispatcher owner is required")
	}
	records, err := d.store.Claim(
		ctx,
		d.config.Owner,
		d.config.BatchSize,
		d.clock.Now(),
		d.config.Lease,
	)
	if err != nil {
		return fmt.Errorf("claim event outbox: %w", err)
	}

	var dispatchErrors []error
	for _, record := range records {
		if err := d.transport.Publish(ctx, record.Envelope); err != nil {
			next := d.clock.Now().Add(retryDelay(record.Attempts))
			rescheduleErr := d.store.Reschedule(ctx, record.ID, d.config.Owner, err.Error(), next)
			dispatchErrors = append(dispatchErrors, errors.Join(err, rescheduleErr))
			continue
		}
		if err := d.store.MarkPublished(ctx, record.ID, d.config.Owner, d.clock.Now()); err != nil {
			dispatchErrors = append(dispatchErrors, fmt.Errorf("mark event %s published: %w", record.ID, err))
		}
	}
	return errors.Join(dispatchErrors...)
}

func (d *OutboxDispatcher) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return fmt.Errorf("outbox dispatcher is closed")
	}
	if d.started {
		return nil
	}
	d.started = true
	d.stop = make(chan struct{})
	d.done = make(chan struct{})
	go d.run(ctx, d.stop, d.done)
	return nil
}

func (d *OutboxDispatcher) run(ctx context.Context, stop <-chan struct{}, done chan struct{}) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		default:
		}
		_ = d.DispatchOnce(ctx)
		timer := time.NewTimer(d.config.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-stop:
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (d *OutboxDispatcher) Close() error {
	d.mu.Lock()
	if !d.closed {
		d.closed = true
		if d.stop != nil {
			close(d.stop)
		}
	}
	done := d.done
	d.mu.Unlock()
	if done == nil {
		return nil
	}
	<-done
	return nil
}

func retryDelay(attempts int) time.Duration {
	switch {
	case attempts <= 1:
		return 5 * time.Second
	case attempts == 2:
		return 30 * time.Second
	case attempts == 3:
		return 5 * time.Minute
	default:
		return 30 * time.Minute
	}
}

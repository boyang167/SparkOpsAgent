package messaging

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"go.uber.org/zap"
)

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type OutboxDispatcherConfig struct {
	Owner          string
	BatchSize      int
	Lease          time.Duration
	PollInterval   time.Duration
	PublishTimeout time.Duration
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
	metrics   *Metrics
	logger    *zap.Logger

	mu          sync.Mutex
	started     bool
	closed      bool
	stop        chan struct{}
	done        chan struct{}
	claimCancel context.CancelFunc
}

func NewOutboxDispatcher(
	store persistence.OutboxStore,
	transport EventTransport,
	config OutboxDispatcherConfig,
	clock Clock,
	metrics *Metrics,
	loggers ...*zap.Logger,
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
	if config.PublishTimeout <= 0 {
		config.PublishTimeout = 30 * time.Second
	}
	if clock == nil {
		clock = systemClock{}
	}
	logger := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	return &OutboxDispatcher{
		store: store, transport: transport, config: config, clock: clock, metrics: metrics, logger: logger,
	}
}

func (d *OutboxDispatcher) DispatchOnce(ctx context.Context) error {
	return d.dispatchOnce(ctx, ctx)
}

func (d *OutboxDispatcher) dispatchOnce(claimCtx, executionCtx context.Context) error {
	if d.store == nil || d.transport == nil {
		return fmt.Errorf("outbox dispatcher dependencies are required")
	}
	if d.config.Owner == "" {
		return fmt.Errorf("outbox dispatcher owner is required")
	}
	var dispatchErrors []error
	if err := d.refreshBacklogMetrics(claimCtx); err != nil {
		d.recordDispatcherError("stats")
		dispatchErrors = append(
			dispatchErrors,
			fmt.Errorf("read event outbox stats: %w", err),
		)
	}
	if err := claimCtx.Err(); err != nil {
		dispatchErrors = append(dispatchErrors, err)
		return errors.Join(dispatchErrors...)
	}
	records, err := d.store.Claim(
		claimCtx,
		d.config.Owner,
		d.config.BatchSize,
		d.clock.Now(),
		d.config.Lease,
	)
	if err != nil {
		d.recordDispatcherError("claim")
		dispatchErrors = append(dispatchErrors, fmt.Errorf("claim event outbox: %w", err))
		return errors.Join(dispatchErrors...)
	}

	for _, record := range records {
		publishCtx, cancelPublish := context.WithTimeout(executionCtx, d.config.PublishTimeout)
		err := d.transport.Publish(publishCtx, record.Envelope)
		cancelPublish()
		if err != nil {
			d.logger.Warn(
				"event outbox publish failed",
				zap.String("event_id", record.ID),
				zap.String("event_type", record.Type),
				zap.Int("attempt", record.Attempts),
				zap.Error(err),
			)
			if d.metrics != nil {
				d.metrics.EventPublish.WithLabelValues("failure").Inc()
			}
			next := d.clock.Now().Add(retryDelay(record.Attempts))
			rescheduleCtx, cancelReschedule := context.WithTimeout(executionCtx, d.config.PublishTimeout)
			rescheduleErr := d.store.Reschedule(rescheduleCtx, record.ID, d.config.Owner, err.Error(), next)
			cancelReschedule()
			if rescheduleErr != nil {
				d.recordDispatcherError("reschedule")
			}
			dispatchErrors = append(dispatchErrors, errors.Join(err, rescheduleErr))
		} else {
			if d.metrics != nil {
				d.metrics.EventPublish.WithLabelValues("success").Inc()
			}
			markCtx, cancelMark := context.WithTimeout(executionCtx, d.config.PublishTimeout)
			markErr := d.store.MarkPublished(markCtx, record.ID, d.config.Owner, d.clock.Now())
			cancelMark()
			if markErr != nil {
				d.logger.Error(
					"event outbox mark published failed",
					zap.String("event_id", record.ID),
					zap.String("event_type", record.Type),
					zap.Int("attempt", record.Attempts),
					zap.Error(markErr),
				)
				d.recordDispatcherError("mark_published")
				dispatchErrors = append(dispatchErrors, fmt.Errorf("mark event %s published: %w", record.ID, markErr))
			} else {
				d.logger.Debug(
					"event outbox published",
					zap.String("event_id", record.ID),
					zap.String("event_type", record.Type),
					zap.Int("attempt", record.Attempts),
				)
			}
		}
		if claimCtx.Err() != nil {
			break
		}
	}
	return errors.Join(dispatchErrors...)
}

func (d *OutboxDispatcher) recordDispatcherError(stage string) {
	if d.metrics != nil {
		d.metrics.EventDispatcherError.WithLabelValues(stage).Inc()
	}
}

func (d *OutboxDispatcher) refreshBacklogMetrics(ctx context.Context) error {
	if d.metrics == nil {
		return nil
	}
	stats, err := d.store.Stats(ctx, d.clock.Now())
	if err != nil {
		return err
	}
	d.metrics.OutboxPending.Set(float64(stats.Pending))
	d.metrics.OutboxOldestSeconds.Set(stats.OldestAge.Seconds())
	return nil
}

func (d *OutboxDispatcher) Start(ctx context.Context) error {
	if err := d.validate(); err != nil {
		return err
	}
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
	claimCtx, claimCancel := context.WithCancel(ctx)
	d.claimCancel = claimCancel
	go d.run(claimCtx, d.stop, d.done)
	return nil
}

func (d *OutboxDispatcher) validate() error {
	if d.store == nil {
		return fmt.Errorf("outbox dispatcher store is required")
	}
	if d.transport == nil {
		return fmt.Errorf("outbox dispatcher transport is required")
	}
	if d.config.Owner == "" {
		return fmt.Errorf("outbox dispatcher owner is required")
	}
	if d.metrics == nil {
		return fmt.Errorf("outbox dispatcher metrics are required")
	}
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
		_ = d.dispatchOnce(ctx, context.WithoutCancel(ctx))
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
	d.StopAccepting()
	d.Wait()
	return nil
}

func (d *OutboxDispatcher) StopAccepting() {
	d.mu.Lock()
	if !d.closed {
		d.closed = true
		if d.claimCancel != nil {
			d.claimCancel()
			d.claimCancel = nil
		}
		if d.stop != nil {
			close(d.stop)
		}
	}
	d.mu.Unlock()
}

func (d *OutboxDispatcher) Wait() {
	d.mu.Lock()
	done := d.done
	d.mu.Unlock()
	if done == nil {
		return
	}
	<-done
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

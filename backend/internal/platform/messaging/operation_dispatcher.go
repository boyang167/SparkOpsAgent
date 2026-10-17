package messaging

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
)

const operationOwnerMaxLength = 160

type OperationDispatcherConfig struct {
	Owner        string
	BatchSize    int
	Lease        time.Duration
	PollInterval time.Duration
	Workers      int
}

type OperationDispatcher struct {
	store    persistence.OperationStore
	registry OperationRegistry
	config   OperationDispatcherConfig
	clock    Clock
	metrics  *Metrics

	mu      sync.Mutex
	started bool
	closed  bool
	stop    chan struct{}
	done    chan struct{}
}

func NewOperationDispatcher(
	store persistence.OperationStore,
	registry OperationRegistry,
	config OperationDispatcherConfig,
	clock Clock,
	metrics *Metrics,
) *OperationDispatcher {
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.Lease <= 0 {
		config.Lease = time.Minute
	}
	if config.PollInterval <= 0 {
		config.PollInterval = time.Second
	}
	if config.Workers <= 0 {
		config.Workers = 1
	}
	if clock == nil {
		clock = systemClock{}
	}
	return &OperationDispatcher{
		store: store, registry: registry, config: config, clock: clock, metrics: metrics,
	}
}

func (d *OperationDispatcher) DispatchOnce(ctx context.Context) error {
	if err := d.validateDependencies(false); err != nil {
		return err
	}
	return d.dispatchOnce(
		ctx,
		d.config.Owner,
		d.config.BatchSize,
	)
}

func (d *OperationDispatcher) dispatchOnce(
	ctx context.Context,
	owner string,
	limit int,
) error {
	var dispatchErrors []error
	if err := d.refreshBacklogMetrics(ctx); err != nil {
		d.recordOperationError("stats", err)
		dispatchErrors = append(dispatchErrors, fmt.Errorf("read operation outbox stats: %w", err))
	}
	records, err := d.store.Claim(
		ctx,
		owner,
		limit,
		d.clock.Now(),
		d.config.Lease,
	)
	if err != nil {
		d.recordOperationError("claim", err)
		dispatchErrors = append(dispatchErrors, fmt.Errorf("claim operation outbox: %w", err))
		return errors.Join(dispatchErrors...)
	}
	for _, record := range records {
		if err := d.dispatch(ctx, owner, record); err != nil {
			dispatchErrors = append(dispatchErrors, err)
		}
	}
	return errors.Join(dispatchErrors...)
}

func (d *OperationDispatcher) dispatch(
	ctx context.Context,
	owner string,
	record persistence.OperationRecord,
) error {
	if !json.Valid(record.Payload) {
		reason := "malformed operation payload"
		markErr := d.store.MarkFailed(
			ctx, record.ID, owner, reason, d.clock.Now(),
		)
		d.recordOperationError("handler", errors.New(reason))
		d.recordOperationError("mark", markErr)
		return errors.Join(errors.New(reason), markErr)
	}
	handler, ok := d.registry.Handler(record.OperationType)
	if !ok {
		reason := fmt.Sprintf("unknown operation type %q", record.OperationType)
		markErr := d.store.MarkFailed(
			ctx, record.ID, owner, reason, d.clock.Now(),
		)
		d.recordOperationError("handler", errors.New(reason))
		d.recordOperationError("mark", markErr)
		return errors.Join(errors.New(reason), markErr)
	}

	operation := Operation{
		ID:             record.ID,
		EventID:        record.EventID,
		Owner:          record.Subscriber,
		OperationType:  record.OperationType,
		IdempotencyKey: record.IdempotencyKey,
		Payload:        record.Payload,
		Attempt:        record.Attempts,
	}
	err := handler(ctx, operation)
	if err == nil {
		if markErr := d.store.MarkCompleted(
			ctx, record.ID, owner, d.clock.Now(),
		); markErr != nil {
			d.recordOperationError("mark", markErr)
			return fmt.Errorf("mark operation %s completed: %w", record.ID, markErr)
		}
		return nil
	}
	d.recordOperationError("handler", err)
	if IsPermanent(err) {
		markErr := d.store.MarkFailed(
			ctx, record.ID, owner, err.Error(), d.clock.Now(),
		)
		d.recordOperationError("mark", markErr)
		return errors.Join(err, markErr)
	}
	next := d.clock.Now().Add(operationRetryDelay(record.Attempts))
	rescheduleErr := d.store.Reschedule(
		ctx, record.ID, owner, err.Error(), next,
	)
	d.recordOperationError("reschedule", rescheduleErr)
	return errors.Join(err, rescheduleErr)
}

func (d *OperationDispatcher) recordOperationError(stage string, err error) {
	if err == nil || d.metrics == nil {
		return
	}
	if errors.Is(err, persistence.ErrLeaseLost) {
		stage = "lease_lost"
	}
	d.metrics.OperationDispatcherError.WithLabelValues(stage).Inc()
}

func (d *OperationDispatcher) refreshBacklogMetrics(ctx context.Context) error {
	if d.metrics == nil {
		return nil
	}
	stats, err := d.store.Stats(ctx, d.clock.Now())
	if err != nil {
		return err
	}
	d.metrics.OperationPending.Set(float64(stats.Pending))
	d.metrics.OperationOldestSeconds.Set(stats.OldestAge.Seconds())
	return nil
}

func (d *OperationDispatcher) Start(ctx context.Context) error {
	if err := d.validateDependencies(true); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return fmt.Errorf("operation dispatcher is closed")
	}
	if d.started {
		return nil
	}
	d.started = true
	d.stop = make(chan struct{})
	d.done = make(chan struct{})
	go d.runWorkers(ctx, d.stop, d.done)
	return nil
}

func (d *OperationDispatcher) validateDependencies(requireMetrics bool) error {
	if d.store == nil {
		return fmt.Errorf("operation dispatcher store is required")
	}
	if d.registry == nil {
		return fmt.Errorf("operation dispatcher registry is required")
	}
	if d.config.Owner == "" {
		return fmt.Errorf("operation dispatcher owner is required")
	}
	if requireMetrics && d.metrics == nil {
		return fmt.Errorf("operation dispatcher metrics are required")
	}
	return nil
}

func (d *OperationDispatcher) runWorkers(
	ctx context.Context,
	stop <-chan struct{},
	done chan struct{},
) {
	var workers sync.WaitGroup
	workers.Add(d.config.Workers)
	for workerIndex := range d.config.Workers {
		owner := operationWorkerOwner(d.config.Owner, workerIndex)
		go func() {
			defer workers.Done()
			d.runWorker(ctx, stop, owner)
		}()
	}
	workers.Wait()
	close(done)
}

func (d *OperationDispatcher) runWorker(
	ctx context.Context,
	stop <-chan struct{},
	owner string,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		default:
		}
		if err := d.dispatchOnce(ctx, owner, 1); err != nil && ctx.Err() != nil {
			return
		}
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

func operationWorkerOwner(instanceOwner string, workerIndex int) string {
	suffix := fmt.Sprintf(".worker-%d", workerIndex+1)
	if len([]rune(instanceOwner))+len([]rune(suffix)) <= operationOwnerMaxLength {
		return instanceOwner + suffix
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(instanceOwner)))[:12]
	reserved := len([]rune(suffix)) + len(digest) + 1
	runes := []rune(instanceOwner)
	runes = runes[:operationOwnerMaxLength-reserved]
	return string(runes) + "-" + digest + suffix
}

func (d *OperationDispatcher) Close() error {
	d.mu.Lock()
	if !d.closed {
		d.closed = true
		if d.stop != nil {
			close(d.stop)
		}
	}
	done := d.done
	d.mu.Unlock()
	if done != nil {
		<-done
	}
	return nil
}

func operationRetryDelay(attempts int) time.Duration {
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

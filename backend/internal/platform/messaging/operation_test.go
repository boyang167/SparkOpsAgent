package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/events"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type operationPayload struct {
	Target string `json:"target"`
}

type operationStoreFake struct {
	mu             sync.Mutex
	records        []persistence.OperationRecord
	claimed        map[string]string
	inserted       []persistence.OperationRecord
	completed      []string
	failed         []string
	failureReasons []string
	rescheduled    []persistence.OperationRecord
	insertErr      error
	claimErr       error
	completeErr    error
	stats          persistence.OperationStats
	statsErr       error
	claims         int
}

func (s *operationStoreFake) Insert(_ context.Context, operation persistence.OperationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.insertErr != nil {
		return s.insertErr
	}
	s.inserted = append(s.inserted, operation)
	return nil
}

func (s *operationStoreFake) Claim(_ context.Context, owner string, limit int, _ time.Time, _ time.Duration) ([]persistence.OperationRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims++
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	if s.claimed == nil {
		s.claimed = make(map[string]string)
	}
	var result []persistence.OperationRecord
	for _, record := range s.records {
		if len(result) == limit {
			break
		}
		if _, exists := s.claimed[record.ID]; exists {
			continue
		}
		s.claimed[record.ID] = owner
		record.LockedBy = owner
		result = append(result, record)
	}
	return result, nil
}

func (s *operationStoreFake) MarkCompleted(_ context.Context, id, owner string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimed[id] != owner {
		return persistence.ErrLeaseLost
	}
	if s.completeErr != nil {
		return s.completeErr
	}
	s.completed = append(s.completed, id)
	return nil
}

func (s *operationStoreFake) MarkFailed(_ context.Context, id, owner, reason string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimed[id] != owner {
		return persistence.ErrLeaseLost
	}
	s.failed = append(s.failed, id)
	s.failureReasons = append(s.failureReasons, reason)
	return nil
}

func (s *operationStoreFake) Reschedule(_ context.Context, id, owner, reason string, next time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimed[id] != owner {
		return persistence.ErrLeaseLost
	}
	for _, record := range s.records {
		if record.ID == id {
			record.LastError = reason
			record.NextAttemptAt = next
			s.rescheduled = append(s.rescheduled, record)
			delete(s.claimed, id)
			return nil
		}
	}
	return persistence.ErrLeaseLost
}

func (s *operationStoreFake) Stats(context.Context, time.Time) (persistence.OperationStats, error) {
	return s.stats, s.statsErr
}

func TestOperationWriterRequiresTransaction(t *testing.T) {
	store := &operationStoreFake{}
	writer := NewOperationWriter(store, fixedClock{now: time.Now().UTC()})

	err := writer.Schedule(context.Background(), OperationRequest{
		Owner: "delivery.command", OperationType: "delivery.queue.publish.v1",
		IdempotencyKey: "command-1:delivery.command:delivery.queue.publish.v1:run-1",
		Payload:        operationPayload{Target: "run-1"},
	})

	require.ErrorContains(t, err, "transaction required")
	require.Empty(t, store.inserted)
}

func TestOperationWriterTreatsDuplicateIdempotencyKeyAsSuccess(t *testing.T) {
	store := &operationStoreFake{insertErr: persistence.ErrOperationIdempotencyConflict}
	writer := NewOperationWriter(store, fixedClock{now: time.Now().UTC()})

	err := withinTestTransaction(t, func(ctx context.Context) error {
		return writer.Schedule(ctx, OperationRequest{
			Owner: "delivery.command", OperationType: "delivery.queue.publish.v1",
			IdempotencyKey: "command-1:delivery.command:delivery.queue.publish.v1:run-1",
			Payload:        operationPayload{Target: "run-1"},
		})
	})

	require.NoError(t, err)
}

func TestOperationWriterDerivesEventMetadataAndValidatesOwner(t *testing.T) {
	now := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	store := &operationStoreFake{}
	writer := NewOperationWriter(store, fixedClock{now: now})
	ctx := events.WithMetadata(context.Background(), events.Metadata{
		EventID: "event-1", Subscriber: "delivery.build-completed.webhook",
	})

	require.NoError(t, withinTestTransactionContext(t, ctx, func(txCtx context.Context) error {
		return writer.Schedule(txCtx, OperationRequest{
			Owner: "delivery.build-completed.webhook", OperationType: "delivery.webhook.send.v1",
			IdempotencyKey: "event-1:delivery.build-completed.webhook:delivery.webhook.send.v1:hook-1",
			Payload:        operationPayload{Target: "hook-1"},
		})
	}))
	require.Len(t, store.inserted, 1)
	require.Equal(t, "event-1", store.inserted[0].EventID)
	require.Equal(t, "delivery.build-completed.webhook", store.inserted[0].Subscriber)
	require.JSONEq(t, `{"target":"hook-1"}`, string(store.inserted[0].Payload))

	err := withinTestTransactionContext(t, ctx, func(txCtx context.Context) error {
		return writer.Schedule(txCtx, OperationRequest{
			Owner: "wrong-owner", OperationType: "delivery.webhook.send.v1",
			IdempotencyKey: "event-1:wrong-owner:delivery.webhook.send.v1:hook-1",
			Payload:        operationPayload{Target: "hook-1"},
		})
	})
	require.ErrorContains(t, err, "owner")
}

func TestOperationWriterCommandUsesOperationIDAsTraceEventID(t *testing.T) {
	store := &operationStoreFake{}
	writer := NewOperationWriter(store, fixedClock{now: time.Now().UTC()})
	key := "command-1:delivery.command:delivery.queue.publish.v1:run-1"

	require.NoError(t, withinTestTransaction(t, func(ctx context.Context) error {
		return writer.Schedule(ctx, OperationRequest{
			Owner: "delivery.command", OperationType: "delivery.queue.publish.v1",
			IdempotencyKey: key, Payload: operationPayload{Target: "run-1"},
		})
	}))

	require.Len(t, store.inserted, 1)
	require.Equal(t, store.inserted[0].ID, store.inserted[0].EventID)
	require.Equal(t, key, store.inserted[0].IdempotencyKey)
}

func TestOperationRegistryRejectsDuplicates(t *testing.T) {
	registry := NewOperationRegistry()
	handler := func(context.Context, Operation) error { return nil }
	require.NoError(t, registry.Register("delivery.webhook.send.v1", handler))
	require.ErrorContains(t, registry.Register("delivery.webhook.send.v1", handler), "duplicate")
	_, ok := registry.Handler("delivery.webhook.send.v1")
	require.True(t, ok)
}

func TestOperationDispatcherRetriesTemporaryFailure(t *testing.T) {
	now := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	record := operationRecordForTest("operation-1", 1, now)
	store := &operationStoreFake{records: []persistence.OperationRecord{record}}
	registry := NewOperationRegistry()
	require.NoError(t, registry.Register(record.OperationType, func(context.Context, Operation) error {
		return errors.New("external service unavailable")
	}))
	dispatcher := newOperationDispatcherForTest(t, store, registry, now, 1)

	err := dispatcher.DispatchOnce(context.Background())

	require.ErrorContains(t, err, "external service unavailable")
	require.Len(t, store.rescheduled, 1)
	require.Equal(t, now.Add(5*time.Second), store.rescheduled[0].NextAttemptAt)
	require.Empty(t, store.completed)
	require.Empty(t, store.failed)
}

func TestOperationDispatcherUsesFixedRetrySchedule(t *testing.T) {
	tests := []struct {
		attempts int
		delay    time.Duration
	}{
		{attempts: 1, delay: 5 * time.Second},
		{attempts: 2, delay: 30 * time.Second},
		{attempts: 3, delay: 5 * time.Minute},
		{attempts: 4, delay: 30 * time.Minute},
		{attempts: 9, delay: 30 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.delay.String(), func(t *testing.T) {
			require.Equal(t, tt.delay, operationRetryDelay(tt.attempts))
		})
	}
}

func TestOperationDispatcherDoesNotRepeatCompletedOperation(t *testing.T) {
	now := time.Now().UTC()
	record := operationRecordForTest("operation-1", 1, now)
	store := &operationStoreFake{records: []persistence.OperationRecord{record}}
	registry := NewOperationRegistry()
	var calls int
	require.NoError(t, registry.Register(record.OperationType, func(context.Context, Operation) error {
		calls++
		return nil
	}))
	dispatcher := newOperationDispatcherForTest(t, store, registry, now, 1)

	require.NoError(t, dispatcher.DispatchOnce(context.Background()))
	store.records = nil // completed rows are not claimable by the real store
	require.NoError(t, dispatcher.DispatchOnce(context.Background()))

	require.Equal(t, 1, calls)
	require.Equal(t, []string{record.ID}, store.completed)
}

func TestOperationDispatcherMarksUnknownOperationPermanent(t *testing.T) {
	now := time.Now().UTC()
	record := operationRecordForTest("operation-1", 1, now)
	store := &operationStoreFake{records: []persistence.OperationRecord{record}}
	dispatcher := newOperationDispatcherForTest(t, store, NewOperationRegistry(), now, 1)

	err := dispatcher.DispatchOnce(context.Background())

	require.ErrorContains(t, err, "unknown operation type")
	require.Equal(t, []string{record.ID}, store.failed)
	require.Contains(t, store.failureReasons[0], record.OperationType)
	require.Empty(t, store.completed)
	require.Empty(t, store.rescheduled)
}

func TestOperationDispatcherMarksMalformedPayloadPermanent(t *testing.T) {
	now := time.Now().UTC()
	record := operationRecordForTest("operation-1", 1, now)
	record.Payload = json.RawMessage(`{"target":`)
	store := &operationStoreFake{records: []persistence.OperationRecord{record}}
	registry := NewOperationRegistry()
	require.NoError(t, registry.Register(record.OperationType, func(context.Context, Operation) error {
		t.Fatal("malformed payload reached handler")
		return nil
	}))
	dispatcher := newOperationDispatcherForTest(t, store, registry, now, 1)

	err := dispatcher.DispatchOnce(context.Background())

	require.ErrorContains(t, err, "malformed operation payload")
	require.Equal(t, []string{record.ID}, store.failed)
}

func TestOperationDispatcherMarksPermanentHandlerFailure(t *testing.T) {
	now := time.Now().UTC()
	record := operationRecordForTest("operation-1", 1, now)
	store := &operationStoreFake{records: []persistence.OperationRecord{record}}
	registry := NewOperationRegistry()
	require.NoError(t, registry.Register(record.OperationType, func(context.Context, Operation) error {
		return Permanent(errors.New("invalid endpoint"))
	}))
	dispatcher := newOperationDispatcherForTest(t, store, registry, now, 1)

	require.ErrorContains(t, dispatcher.DispatchOnce(context.Background()), "invalid endpoint")
	require.Equal(t, []string{record.ID}, store.failed)
	require.Empty(t, store.rescheduled)
}

func TestOperationDispatcherMayReexecuteAfterSuccessBeforeMarkCompleted(t *testing.T) {
	now := time.Now().UTC()
	record := operationRecordForTest("operation-1", 1, now)
	store := &operationStoreFake{
		records:     []persistence.OperationRecord{record},
		completeErr: errors.New("database unavailable after external success"),
	}
	registry := NewOperationRegistry()
	var keys []string
	require.NoError(t, registry.Register(record.OperationType, func(_ context.Context, operation Operation) error {
		keys = append(keys, operation.IdempotencyKey)
		return nil
	}))
	dispatcher := newOperationDispatcherForTest(t, store, registry, now, 1)

	require.ErrorContains(t, dispatcher.DispatchOnce(context.Background()), "database unavailable")
	store.mu.Lock()
	delete(store.claimed, record.ID) // simulate expired lease recovery
	store.completeErr = nil
	store.mu.Unlock()
	require.NoError(t, dispatcher.DispatchOnce(context.Background()))

	require.Equal(t, []string{record.IdempotencyKey, record.IdempotencyKey}, keys)
	require.Equal(t, []string{record.ID}, store.completed)
}

func TestOperationDispatcherUpdatesMetricsWithoutBlockingOnStatsFailure(t *testing.T) {
	now := time.Now().UTC()
	record := operationRecordForTest("operation-1", 1, now)
	store := &operationStoreFake{
		records: []persistence.OperationRecord{record},
		stats:   persistence.OperationStats{Pending: 4, OldestAge: 3 * time.Minute},
	}
	registry := NewOperationRegistry()
	require.NoError(t, registry.Register(record.OperationType, func(context.Context, Operation) error { return nil }))
	metrics := newDispatcherTestMetrics(t)
	dispatcher := NewOperationDispatcher(store, registry, OperationDispatcherConfig{
		Owner: "worker-a", BatchSize: 1, Lease: time.Minute, Workers: 1,
	}, fixedClock{now: now}, metrics)

	require.NoError(t, dispatcher.DispatchOnce(context.Background()))
	require.Equal(t, float64(4), prometheusMetricValue(t, metrics.OperationPending))
	require.Equal(t, float64(180), prometheusMetricValue(t, metrics.OperationOldestSeconds))

	store.statsErr = errors.New("stats unavailable")
	store.records = []persistence.OperationRecord{operationRecordForTest("operation-2", 1, now)}
	err := dispatcher.DispatchOnce(context.Background())
	require.ErrorContains(t, err, "stats unavailable")
	require.Contains(t, store.completed, "operation-2")
}

func TestOperationDispatcherRunsConcurrentWorkersAndDrainsOnClose(t *testing.T) {
	now := time.Now().UTC()
	store := &operationStoreFake{records: []persistence.OperationRecord{
		operationRecordForTest("operation-1", 1, now),
		operationRecordForTest("operation-2", 1, now),
	}}
	registry := NewOperationRegistry()
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var active atomic.Int32
	var maximum atomic.Int32
	require.NoError(t, registry.Register("delivery.webhook.send.v1", func(context.Context, Operation) error {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		started <- struct{}{}
		<-release
		return nil
	}))
	dispatcher := NewOperationDispatcher(store, registry, OperationDispatcherConfig{
		Owner: "worker-a", BatchSize: 1, Lease: time.Minute,
		PollInterval: time.Hour, Workers: 2,
	}, fixedClock{now: now}, newDispatcherTestMetrics(t))
	require.NoError(t, dispatcher.Start(context.Background()))
	<-started
	<-started

	closed := make(chan struct{})
	go func() {
		_ = dispatcher.Close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("Close returned before in-flight operations completed")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	require.Eventually(t, func() bool {
		select {
		case <-closed:
			return true
		default:
			return false
		}
	}, time.Second, time.Millisecond)
	require.Equal(t, int32(2), maximum.Load())
	claimsAfterClose := store.claims
	time.Sleep(10 * time.Millisecond)
	require.Equal(t, claimsAfterClose, store.claims)
}

func newOperationDispatcherForTest(
	t *testing.T,
	store persistence.OperationStore,
	registry OperationRegistry,
	now time.Time,
	workers int,
) *OperationDispatcher {
	t.Helper()
	return NewOperationDispatcher(store, registry, OperationDispatcherConfig{
		Owner: "worker-a", BatchSize: 10, Lease: time.Minute, Workers: workers,
	}, fixedClock{now: now}, prometheusOperationMetrics(t))
}

func prometheusOperationMetrics(t *testing.T) *Metrics {
	t.Helper()
	metrics, err := NewMetrics(prometheus.NewRegistry())
	require.NoError(t, err)
	return metrics
}

func operationRecordForTest(id string, attempts int, now time.Time) persistence.OperationRecord {
	return persistence.OperationRecord{
		ID: id, EventID: "event-1", Subscriber: "delivery.build-completed.webhook",
		OperationType:  "delivery.webhook.send.v1",
		IdempotencyKey: "event-1:delivery.build-completed.webhook:delivery.webhook.send.v1:hook-1",
		Payload:        json.RawMessage(`{"target":"hook-1"}`), Attempts: attempts,
		NextAttemptAt: now, CreatedAt: now,
	}
}

func withinTestTransaction(t *testing.T, fn func(context.Context) error) error {
	t.Helper()
	return withinTestTransactionContext(t, context.Background(), fn)
}

func withinTestTransactionContext(t *testing.T, ctx context.Context, fn func(context.Context) error) error {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	return postgres.NewTransactor(db).WithinTransaction(ctx, fn)
}

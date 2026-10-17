package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
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

type operationContextTestKey struct{}

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
	failErr        error
	rescheduleErr  error
	stats          persistence.OperationStats
	statsErr       error
	claims         int
	claimOwners    []string
	claimLimits    []int
	completedOwner []string
	markContextErr []error
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
	s.claimOwners = append(s.claimOwners, owner)
	s.claimLimits = append(s.claimLimits, limit)
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

func (s *operationStoreFake) MarkCompleted(ctx context.Context, id, owner string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimed[id] != owner {
		return persistence.ErrLeaseLost
	}
	if s.completeErr != nil {
		return s.completeErr
	}
	s.completed = append(s.completed, id)
	s.completedOwner = append(s.completedOwner, owner)
	s.markContextErr = append(s.markContextErr, ctx.Err())
	return nil
}

func (s *operationStoreFake) MarkFailed(_ context.Context, id, owner, reason string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimed[id] != owner {
		return persistence.ErrLeaseLost
	}
	if s.failErr != nil {
		return s.failErr
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
	if s.rescheduleErr != nil {
		return s.rescheduleErr
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

func TestOperationWriterDoesNotSwallowIdempotencyConflict(t *testing.T) {
	store := &operationStoreFake{insertErr: persistence.ErrOperationIdempotencyConflict}
	writer := NewOperationWriter(store, fixedClock{now: time.Now().UTC()})

	err := withinTestTransaction(t, func(ctx context.Context) error {
		return writer.Schedule(ctx, OperationRequest{
			Owner: "delivery.command", OperationType: "delivery.queue.publish.v1",
			IdempotencyKey: "command-1:delivery.command:delivery.queue.publish.v1:run-1",
			Payload:        operationPayload{Target: "run-1"},
		})
	})

	require.ErrorIs(t, err, persistence.ErrOperationIdempotencyConflict)
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

func TestOperationWriterCommandUsesOperationUUIDAsTraceEventID(t *testing.T) {
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
	_, err := uuid.Parse(store.inserted[0].EventID)
	require.NoError(t, err)
	require.Equal(t, key, store.inserted[0].IdempotencyKey)
}

func TestOperationWriterValidatesCommandIdempotencyKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "empty event id", key: ":delivery.command:delivery.queue.publish.v1:run-1"},
		{name: "empty owner", key: "command-1::delivery.queue.publish.v1:run-1"},
		{name: "wrong owner", key: "command-1:other.command:delivery.queue.publish.v1:run-1"},
		{name: "empty type", key: "command-1:delivery.command::run-1"},
		{name: "wrong type", key: "command-1:delivery.command:other.operation:run-1"},
		{name: "empty target", key: "command-1:delivery.command:delivery.queue.publish.v1:"},
		{name: "too few segments", key: "command-1:delivery.command:delivery.queue.publish.v1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &operationStoreFake{}
			writer := NewOperationWriter(store, fixedClock{now: time.Now().UTC()})
			err := withinTestTransaction(t, func(ctx context.Context) error {
				return writer.Schedule(ctx, OperationRequest{
					Owner: "delivery.command", OperationType: "delivery.queue.publish.v1",
					IdempotencyKey: tt.key, Payload: operationPayload{Target: "run-1"},
				})
			})
			require.ErrorContains(t, err, "idempotency key")
			require.Empty(t, store.inserted)
		})
	}
}

func TestOperationWriterAllowsColonInTargetID(t *testing.T) {
	store := &operationStoreFake{}
	writer := NewOperationWriter(store, fixedClock{now: time.Now().UTC()})
	key := "command-1:delivery.command:delivery.queue.publish.v1:https://cluster:6443/api"

	require.NoError(t, withinTestTransaction(t, func(ctx context.Context) error {
		return writer.Schedule(ctx, OperationRequest{
			Owner: "delivery.command", OperationType: "delivery.queue.publish.v1",
			IdempotencyKey: key, Payload: operationPayload{Target: "https://cluster:6443/api"},
		})
	}))
	require.Len(t, store.inserted, 1)
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

func TestOperationDispatcherWorkersUseUniqueBoundedOwnersAndFenceExpiredLease(t *testing.T) {
	now := time.Now().UTC()
	record := operationRecordForTest("operation-1", 1, now)
	firstStarted := make(chan struct{})
	store := &reclaimingOperationStore{
		operationStoreFake: operationStoreFake{records: []persistence.OperationRecord{record}},
		allowReclaim:       firstStarted,
	}
	registry := NewOperationRegistry()
	releaseFirst := make(chan struct{})
	var calls atomic.Int32
	require.NoError(t, registry.Register(record.OperationType, func(context.Context, Operation) error {
		if calls.Add(1) == 1 {
			close(firstStarted)
			<-releaseFirst
		}
		return nil
	}))
	metrics := newDispatcherTestMetrics(t)
	dispatcher := NewOperationDispatcher(store, registry, OperationDispatcherConfig{
		Owner:        strings.Repeat("instance-owner-", 20),
		Workers:      2,
		PollInterval: time.Hour,
		Lease:        time.Millisecond,
	}, fixedClock{now: now}, metrics)
	require.NoError(t, dispatcher.Start(context.Background()))
	<-firstStarted
	require.Eventually(t, func() bool {
		store.mu.Lock()
		defer store.mu.Unlock()
		return len(store.completed) == 1
	}, time.Second, time.Millisecond)
	close(releaseFirst)
	require.Eventually(t, func() bool {
		return prometheusMetricValue(
			t,
			metrics.OperationDispatcherError.WithLabelValues("lease_lost"),
		) == 1
	}, time.Second, time.Millisecond)
	require.NoError(t, dispatcher.Close())

	store.mu.Lock()
	defer store.mu.Unlock()
	require.Len(t, store.claimOwners, 2)
	require.NotEqual(t, store.claimOwners[0], store.claimOwners[1])
	require.LessOrEqual(t, len(store.claimOwners[0]), operationOwnerMaxLength)
	require.LessOrEqual(t, len(store.claimOwners[1]), operationOwnerMaxLength)
	require.Len(t, store.completed, 1)
	require.Equal(t, store.claimOwners[1], store.completedOwner[0])
}

func TestOperationDispatcherInstancesWithSameConfigUseDifferentWorkerOwners(t *testing.T) {
	now := time.Now().UTC()
	store := &operationStoreFake{records: []persistence.OperationRecord{
		operationRecordForTest("operation-1", 1, now),
		operationRecordForTest("operation-2", 1, now),
	}}
	registry := NewOperationRegistry()
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	require.NoError(t, registry.Register("delivery.webhook.send.v1", func(context.Context, Operation) error {
		started <- struct{}{}
		<-release
		return nil
	}))
	config := OperationDispatcherConfig{
		Owner: "instance-a", Workers: 1, PollInterval: time.Hour,
	}
	first := NewOperationDispatcher(
		store, registry, config, fixedClock{now: now}, newDispatcherTestMetrics(t),
	)
	second := NewOperationDispatcher(
		store, registry, config, fixedClock{now: now}, newDispatcherTestMetrics(t),
	)
	_, err := uuid.Parse(first.runtimeID)
	require.NoError(t, err)
	_, err = uuid.Parse(second.runtimeID)
	require.NoError(t, err)
	require.NotEqual(t, first.runtimeID, second.runtimeID)
	require.NoError(t, first.Start(context.Background()))
	require.NoError(t, second.Start(context.Background()))
	<-started
	<-started

	store.mu.Lock()
	require.Len(t, store.claimOwners, 2)
	require.NotEqual(t, store.claimOwners[0], store.claimOwners[1])
	store.mu.Unlock()
	close(release)
	require.NoError(t, first.Close())
	require.NoError(t, second.Close())
}

func TestOperationDispatcherOldInstanceCannotOverwriteRecoveredLease(t *testing.T) {
	now := time.Now().UTC()
	record := operationRecordForTest("operation-1", 1, now)
	firstStarted := make(chan struct{})
	store := &reclaimingOperationStore{
		operationStoreFake: operationStoreFake{records: []persistence.OperationRecord{record}},
		allowReclaim:       firstStarted,
	}
	registry := NewOperationRegistry()
	releaseFirst := make(chan struct{})
	var calls atomic.Int32
	require.NoError(t, registry.Register(record.OperationType, func(context.Context, Operation) error {
		if calls.Add(1) == 1 {
			close(firstStarted)
			<-releaseFirst
		}
		return nil
	}))
	config := OperationDispatcherConfig{
		Owner: "instance-a", Workers: 1, PollInterval: time.Hour,
	}
	firstMetrics := newDispatcherTestMetrics(t)
	first := NewOperationDispatcher(
		store, registry, config, fixedClock{now: now}, firstMetrics,
	)
	second := NewOperationDispatcher(
		store, registry, config, fixedClock{now: now}, newDispatcherTestMetrics(t),
	)
	require.NoError(t, first.Start(context.Background()))
	<-firstStarted
	require.NoError(t, second.Start(context.Background()))
	require.Eventually(t, func() bool {
		store.mu.Lock()
		defer store.mu.Unlock()
		return len(store.completed) == 1
	}, time.Second, time.Millisecond)
	close(releaseFirst)
	require.Eventually(t, func() bool {
		return prometheusMetricValue(
			t,
			firstMetrics.OperationDispatcherError.WithLabelValues("lease_lost"),
		) == 1
	}, time.Second, time.Millisecond)
	require.NoError(t, first.Close())
	require.NoError(t, second.Close())

	store.mu.Lock()
	defer store.mu.Unlock()
	require.Len(t, store.completed, 1)
	require.Len(t, store.claimOwners, 2)
	require.NotEqual(t, store.claimOwners[0], store.claimOwners[1])
}

func TestOperationDispatcherRuntimeWorkersClaimOnlyOneRecord(t *testing.T) {
	now := time.Now().UTC()
	store := &operationStoreFake{records: []persistence.OperationRecord{
		operationRecordForTest("operation-1", 1, now),
		operationRecordForTest("operation-2", 1, now),
		operationRecordForTest("operation-3", 1, now),
	}}
	registry := NewOperationRegistry()
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	require.NoError(t, registry.Register("delivery.webhook.send.v1", func(context.Context, Operation) error {
		started <- struct{}{}
		<-release
		return nil
	}))
	dispatcher := NewOperationDispatcher(store, registry, OperationDispatcherConfig{
		Owner: "instance-a", Workers: 2, PollInterval: time.Hour,
	}, fixedClock{now: now}, newDispatcherTestMetrics(t))
	require.NoError(t, dispatcher.Start(context.Background()))
	<-started
	secondStarted := false
	select {
	case <-started:
		secondStarted = true
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	require.NoError(t, dispatcher.Close())

	require.True(t, secondStarted, "two workers should execute concurrently")
	store.mu.Lock()
	defer store.mu.Unlock()
	require.NotEmpty(t, store.claimLimits)
	for _, limit := range store.claimLimits {
		require.Equal(t, 1, limit)
	}
	require.Len(t, store.claimed, 2, "third operation must not be pre-claimed")
	require.NotEqual(t, store.claimOwners[0], store.claimOwners[1])
}

func TestOperationDispatcherParentCancelAndRepeatedClose(t *testing.T) {
	parent := context.WithValue(
		context.Background(),
		operationContextTestKey{},
		"retained-value",
	)
	ctx, cancel := context.WithCancel(parent)
	now := time.Now().UTC()
	record := operationRecordForTest("operation-1", 1, now)
	store := &operationStoreFake{records: []persistence.OperationRecord{record}}
	registry := NewOperationRegistry()
	started := make(chan struct{})
	release := make(chan struct{})
	handlerContextErr := make(chan error, 1)
	handlerContextValue := make(chan any, 1)
	require.NoError(t, registry.Register(record.OperationType, func(ctx context.Context, _ Operation) error {
		close(started)
		<-release
		handlerContextErr <- ctx.Err()
		handlerContextValue <- ctx.Value(operationContextTestKey{})
		return nil
	}))
	dispatcher := NewOperationDispatcher(
		store,
		registry,
		OperationDispatcherConfig{
			Owner: "instance-a", Workers: 2, PollInterval: time.Hour,
		},
		fixedClock{now: now},
		newDispatcherTestMetrics(t),
	)
	require.NoError(t, dispatcher.Start(ctx))
	<-started
	cancel()

	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() { first <- dispatcher.Close() }()
	go func() { second <- dispatcher.Close() }()
	select {
	case <-first:
		t.Fatal("Close returned before in-flight handler completed")
	case <-second:
		t.Fatal("repeated Close returned before in-flight handler completed")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-handlerContextErr)
	require.Equal(t, "retained-value", <-handlerContextValue)
	require.NoError(t, <-first)
	require.NoError(t, <-second)
	require.NoError(t, dispatcher.Close())
	require.Equal(t, []string{record.ID}, store.completed)
	require.Len(t, store.markContextErr, 1)
	require.NoError(t, store.markContextErr[0])
}

func TestOperationDispatcherCloseCancelsBlockedClaim(t *testing.T) {
	store := &blockingClaimOperationStore{
		operationStoreFake: operationStoreFake{},
		entered:            make(chan struct{}),
		forceRelease:       make(chan struct{}),
	}
	registry := NewOperationRegistry()
	var handlerCalls atomic.Int32
	require.NoError(t, registry.Register(
		"delivery.webhook.send.v1",
		func(context.Context, Operation) error {
			handlerCalls.Add(1)
			return nil
		},
	))
	dispatcher := NewOperationDispatcher(
		store,
		registry,
		OperationDispatcherConfig{
			Owner: "instance-a", Workers: 1, PollInterval: time.Hour,
		},
		fixedClock{now: time.Now().UTC()},
		newDispatcherTestMetrics(t),
	)
	require.NoError(t, dispatcher.Start(context.Background()))
	<-store.entered

	closed := make(chan error, 1)
	go func() { closed <- dispatcher.Close() }()
	var closeErr error
	returned := false
	select {
	case closeErr = <-closed:
		returned = true
	case <-time.After(100 * time.Millisecond):
		close(store.forceRelease)
		closeErr = <-closed
	}

	require.True(t, returned, "Close must cancel a blocked Claim")
	require.NoError(t, closeErr)
	require.Zero(t, handlerCalls.Load())
}

func TestOperationDispatcherParentCancelCancelsBlockedClaim(t *testing.T) {
	store := &blockingClaimOperationStore{
		operationStoreFake: operationStoreFake{},
		entered:            make(chan struct{}),
		forceRelease:       make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	dispatcher := NewOperationDispatcher(
		store,
		NewOperationRegistry(),
		OperationDispatcherConfig{
			Owner: "instance-a", Workers: 1, PollInterval: time.Hour,
		},
		fixedClock{now: time.Now().UTC()},
		newDispatcherTestMetrics(t),
	)
	require.NoError(t, dispatcher.Start(ctx))
	<-store.entered

	cancel()
	closed := make(chan error, 1)
	go func() { closed <- dispatcher.Close() }()
	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(time.Second):
		close(store.forceRelease)
		t.Fatal("parent cancellation did not interrupt blocked Claim")
	}
}

func TestOperationDispatcherCloseDuringStatsDoesNotStartClaim(t *testing.T) {
	now := time.Now().UTC()
	store := &blockingStatsOperationStore{
		operationStoreFake: operationStoreFake{
			records: []persistence.OperationRecord{
				operationRecordForTest("operation-1", 1, now),
			},
		},
		entered:      make(chan struct{}),
		forceRelease: make(chan struct{}),
	}
	registry := NewOperationRegistry()
	var handlerCalls atomic.Int32
	require.NoError(t, registry.Register(
		"delivery.webhook.send.v1",
		func(context.Context, Operation) error {
			handlerCalls.Add(1)
			return nil
		},
	))
	dispatcher := NewOperationDispatcher(
		store,
		registry,
		OperationDispatcherConfig{
			Owner: "instance-a", Workers: 1, PollInterval: time.Hour,
		},
		fixedClock{now: now},
		newDispatcherTestMetrics(t),
	)
	require.NoError(t, dispatcher.Start(context.Background()))
	<-store.entered

	closed := make(chan error, 1)
	go func() { closed <- dispatcher.Close() }()
	var closeErr error
	returned := false
	select {
	case closeErr = <-closed:
		returned = true
	case <-time.After(100 * time.Millisecond):
		close(store.forceRelease)
		closeErr = <-closed
	}

	require.True(t, returned, "Close must cancel blocked Stats")
	require.NoError(t, closeErr)
	require.Zero(t, store.claimCalls.Load())
	require.Zero(t, handlerCalls.Load())
}

func TestOperationDispatcherRecordsBoundedErrorStagesOnce(t *testing.T) {
	now := time.Now().UTC()
	record := operationRecordForTest("operation-1", 1, now)
	tests := []struct {
		name       string
		store      *operationStoreFake
		handler    OperationHandler
		wantStages map[string]float64
	}{
		{
			name: "stats side path",
			store: &operationStoreFake{
				records:  []persistence.OperationRecord{record},
				statsErr: errors.New("stats unavailable"),
			},
			handler:    func(context.Context, Operation) error { return nil },
			wantStages: map[string]float64{"stats": 1},
		},
		{
			name:       "claim",
			store:      &operationStoreFake{claimErr: errors.New("claim unavailable")},
			handler:    func(context.Context, Operation) error { return nil },
			wantStages: map[string]float64{"claim": 1},
		},
		{
			name:       "handler",
			store:      &operationStoreFake{records: []persistence.OperationRecord{record}},
			handler:    func(context.Context, Operation) error { return errors.New("temporary") },
			wantStages: map[string]float64{"handler": 1},
		},
		{
			name: "mark",
			store: &operationStoreFake{
				records:     []persistence.OperationRecord{record},
				completeErr: errors.New("mark unavailable"),
			},
			handler:    func(context.Context, Operation) error { return nil },
			wantStages: map[string]float64{"mark": 1},
		},
		{
			name: "reschedule",
			store: &operationStoreFake{
				records:       []persistence.OperationRecord{record},
				rescheduleErr: errors.New("reschedule unavailable"),
			},
			handler:    func(context.Context, Operation) error { return errors.New("temporary") },
			wantStages: map[string]float64{"handler": 1, "reschedule": 1},
		},
		{
			name: "lease lost",
			store: &operationStoreFake{
				records:     []persistence.OperationRecord{record},
				completeErr: persistence.ErrLeaseLost,
			},
			handler:    func(context.Context, Operation) error { return nil },
			wantStages: map[string]float64{"lease_lost": 1},
		},
	}
	allStages := []string{"stats", "claim", "handler", "mark", "reschedule", "lease_lost"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewOperationRegistry()
			require.NoError(t, registry.Register(record.OperationType, tt.handler))
			metrics := newDispatcherTestMetrics(t)
			dispatcher := NewOperationDispatcher(
				tt.store,
				registry,
				OperationDispatcherConfig{Owner: "instance-a", BatchSize: 1},
				fixedClock{now: now},
				metrics,
			)

			_ = dispatcher.DispatchOnce(context.Background())

			for _, stage := range allStages {
				require.Equal(
					t,
					tt.wantStages[stage],
					prometheusMetricValue(
						t,
						metrics.OperationDispatcherError.WithLabelValues(stage),
					),
					stage,
				)
			}
		})
	}
}

func TestOperationDispatcherBackgroundLoopObservesDispatchErrors(t *testing.T) {
	store := &operationStoreFake{claimErr: errors.New("database unavailable")}
	metrics := newDispatcherTestMetrics(t)
	dispatcher := NewOperationDispatcher(
		store,
		NewOperationRegistry(),
		OperationDispatcherConfig{
			Owner: "instance-a", Workers: 1, PollInterval: time.Millisecond,
		},
		fixedClock{now: time.Now().UTC()},
		metrics,
	)

	require.NoError(t, dispatcher.Start(context.Background()))
	require.Eventually(t, func() bool {
		return prometheusMetricValue(
			t,
			metrics.OperationDispatcherError.WithLabelValues("claim"),
		) > 0
	}, time.Second, time.Millisecond)
	require.NoError(t, dispatcher.Close())
}

type reclaimingOperationStore struct {
	operationStoreFake
	claimCalls   int
	allowReclaim <-chan struct{}
}

type blockingClaimOperationStore struct {
	operationStoreFake
	entered      chan struct{}
	forceRelease chan struct{}
	once         sync.Once
}

func (s *blockingClaimOperationStore) Claim(
	ctx context.Context,
	_ string,
	_ int,
	_ time.Time,
	_ time.Duration,
) ([]persistence.OperationRecord, error) {
	s.once.Do(func() { close(s.entered) })
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.forceRelease:
		return nil, nil
	}
}

type blockingStatsOperationStore struct {
	operationStoreFake
	entered      chan struct{}
	forceRelease chan struct{}
	once         sync.Once
	claimCalls   atomic.Int32
}

func (s *blockingStatsOperationStore) Stats(
	ctx context.Context,
	_ time.Time,
) (persistence.OperationStats, error) {
	s.once.Do(func() { close(s.entered) })
	select {
	case <-ctx.Done():
		return persistence.OperationStats{}, ctx.Err()
	case <-s.forceRelease:
		return persistence.OperationStats{}, errors.New("forced stats release")
	}
}

func (s *blockingStatsOperationStore) Claim(
	_ context.Context,
	owner string,
	limit int,
	now time.Time,
	lease time.Duration,
) ([]persistence.OperationRecord, error) {
	s.claimCalls.Add(1)
	return s.operationStoreFake.Claim(
		context.Background(),
		owner,
		limit,
		now,
		lease,
	)
}

func (s *reclaimingOperationStore) Claim(
	_ context.Context,
	owner string,
	limit int,
	_ time.Time,
	_ time.Duration,
) ([]persistence.OperationRecord, error) {
	s.mu.Lock()
	s.claimCalls++
	call := s.claimCalls
	s.claims++
	s.mu.Unlock()
	if call == 2 && s.allowReclaim != nil {
		<-s.allowReclaim
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimOwners = append(s.claimOwners, owner)
	s.claimLimits = append(s.claimLimits, limit)
	if call > 2 {
		return nil, nil
	}
	record := s.records[0]
	s.claimed = map[string]string{record.ID: owner}
	record.LockedBy = owner
	record.Attempts = call
	return []persistence.OperationRecord{record}, nil
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
